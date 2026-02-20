package scanner

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// workItem is a unit of work for the directory scanner pool.
type workItem struct {
	node    *Node
	parent  *Node           // nil for root; size is propagated to parent when done
	itemWg  *sync.WaitGroup // per-item, Done() called when THIS dir is finished
	totalWg *sync.WaitGroup // global, Add/Done mirrors itemWg for root waiter
}

// Scan walks the directory tree rooted at root concurrently using a bounded
// worker pool (NumCPU*2 goroutines) so that scanning very deep or wide trees
// does not create tens of thousands of goroutines.
//
// progressCh (optional) receives byte counts as files are encountered.
// Sends are non-blocking — if the consumer is slow, progress ticks are dropped
// rather than stalling a scanner worker. The channel is NOT closed by Scan;
// the caller should close it after use.
func Scan(ctx context.Context, root string, progressCh chan<- int64) (*Node, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	info, err := os.Lstat(absRoot)
	if err != nil {
		return nil, err
	}

	rootNode := &Node{
		Name:  filepath.Base(absRoot),
		Path:  absRoot,
		IsDir: info.IsDir(),
	}

	if !info.IsDir() {
		rootNode.SetSize(info.Size())
		sendProgress(ctx, progressCh, info.Size())
		return rootNode, nil
	}

	// Bounded worker pool: cap goroutines at NumCPU*2 (min 4).
	numWorkers := runtime.NumCPU() * 2
	if numWorkers < 4 {
		numWorkers = 4
	}

	// Queue depth: large buffer avoids workers starving while items are being
	// enqueued. numWorkers*32 is generous without being wasteful.
	queue := make(chan workItem, numWorkers*32)

	// Workers drain the queue until it is closed.
	var poolWg sync.WaitGroup
	for range numWorkers {
		poolWg.Add(1)
		go func() {
			defer poolWg.Done()
			for item := range queue {
				processDir(ctx, item, queue, progressCh)
			}
		}()
	}

	// totalWg tracks every directory still in-flight across the whole tree.
	var totalWg sync.WaitGroup
	rootItemWg := &sync.WaitGroup{}

	totalWg.Add(1)
	rootItemWg.Add(1)
	queue <- workItem{
		node:    rootNode,
		parent:  nil,
		itemWg:  rootItemWg,
		totalWg: &totalWg,
	}

	// Block until every directory in the tree has been processed.
	totalWg.Wait()
	close(queue)
	poolWg.Wait()

	return rootNode, nil
}

// processDir reads a single directory, handles its file children inline, and
// enqueues subdirectory children as new work items. When it returns it signals
// both itemWg and totalWg and propagates its accumulated size to the parent.
func processDir(ctx context.Context, item workItem, queue chan<- workItem, progressCh chan<- int64) {
	defer func() {
		if item.parent != nil {
			item.parent.AddSize(item.node.Size())
		}
		item.itemWg.Done()
		item.totalWg.Done()
	}()

	if ctx.Err() != nil {
		return
	}

	entries, err := os.ReadDir(item.node.Path)
	if err != nil {
		item.node.Err = err
		return
	}

	// Pre-allocate to the exact entry count to avoid repeated slice reallocs
	// for directories with many entries (e.g. node_modules with 50 000 files).
	item.node.Children = make([]*Node, 0, len(entries))

	// childrenWg: tracks subdirectory items we enqueue so we can wait for
	// their sizes to be propagated back before we call our own Done().
	var childrenWg sync.WaitGroup

	for _, entry := range entries {
		if ctx.Err() != nil {
			break
		}

		entryPath := filepath.Join(item.node.Path, entry.Name())
		child := &Node{
			Name:  entry.Name(),
			Path:  entryPath,
			IsDir: entry.IsDir(),
		}

		// Never follow symlinks — avoids infinite cycles.
		if entry.Type()&fs.ModeSymlink != 0 {
			child.IsDir = false
			item.node.Children = append(item.node.Children, child)
			continue
		}

		if entry.IsDir() {
			item.node.Children = append(item.node.Children, child)

			childWg := &sync.WaitGroup{}
			childWg.Add(1)
			childrenWg.Add(1)
			item.totalWg.Add(1)

			queue <- workItem{
				node:    child,
				parent:  item.node,
				itemWg:  childWg,
				totalWg: item.totalWg,
			}

			// When the child finishes (its itemWg reaches zero), signal our
			// childrenWg so the parent wait below can unblock.
			go func(cwg *sync.WaitGroup) {
				cwg.Wait()
				childrenWg.Done()
			}(childWg)
		} else {
			info, err := entry.Info()
			if err != nil {
				item.node.Children = append(item.node.Children, child)
				continue
			}
			size := info.Size()
			child.SetSize(size)
			item.node.AddSize(size)
			sendProgress(ctx, progressCh, size)
			item.node.Children = append(item.node.Children, child)
		}
	}

	// Wait for all subdirectory sizes to be accumulated before propagating our
	// own size upward. This ensures parent sizes are correct.
	childrenWg.Wait()
}

// sendProgress sends sz to progressCh without blocking. If the channel is full
// or nil the tick is silently dropped — progress is best-effort and must never
// stall a scanner worker.
func sendProgress(ctx context.Context, ch chan<- int64, sz int64) {
	if ch == nil || ctx.Err() != nil {
		return
	}
	select {
	case ch <- sz:
	default: // consumer is slow; drop rather than block
	}
}
