package scanner

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// Scan walks the directory tree rooted at root concurrently using a semaphore-
// limited goroutine-per-directory model. A buffered semaphore caps the number
// of goroutines simultaneously performing directory I/O (NumCPU*2, min 4),
// preventing both goroutine explosion on very wide/deep trees and the
// bounded-channel deadlock that occurred in the previous bounded worker-pool
// design (where all workers could block trying to enqueue new work into the
// same full channel they were consuming from).
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

	// Semaphore caps concurrent directory I/O goroutines.
	numWorkers := runtime.NumCPU() * 2
	if numWorkers < 4 {
		numWorkers = 4
	}
	sem := make(chan struct{}, numWorkers)

	// globalWg tracks every in-flight scanDir goroutine so Scan can wait for
	// the entire tree to finish before returning.
	var globalWg sync.WaitGroup
	globalWg.Add(1)
	go scanDir(ctx, rootNode, nil, nil, sem, progressCh, &globalWg)
	globalWg.Wait()

	return rootNode, nil
}

// scanDir reads a single directory, processes its file children inline, and
// spawns a new goroutine (bounded by sem) for each subdirectory child.
//
// It acquires the semaphore only for the os.ReadDir call, then releases it
// immediately so that child goroutines can also acquire it. This prevents the
// deadlock where holding the semaphore while enqueuing children causes
// starvation when the semaphore is exhausted.
//
// parentWg is the WaitGroup belonging to this node's *parent* — Done() is
// called on it when this goroutine finishes so the parent can wait for all
// its direct children before propagating its own size upward.
func scanDir(
	ctx context.Context,
	node *Node,
	parent *Node,
	parentWg *sync.WaitGroup, // signal parent when done
	sem chan struct{},
	progressCh chan<- int64,
	globalWg *sync.WaitGroup,
) {
	defer func() {
		// Propagate our accumulated size to the parent.
		if parent != nil {
			parent.AddSize(node.Size())
		}
		// Signal our parent's childrenWg (if any) that we are done.
		if parentWg != nil {
			parentWg.Done()
		}
		globalWg.Done()
	}()

	if ctx.Err() != nil {
		return
	}

	// Acquire semaphore before doing directory I/O; release immediately after.
	// Releasing before spawning children means children can also acquire the
	// semaphore without deadlocking.
	sem <- struct{}{}
	entries, err := os.ReadDir(node.Path)
	<-sem

	if err != nil {
		node.Err = err
		return
	}

	// Pre-allocate to the exact entry count to avoid repeated slice reallocs
	// for directories with many entries (e.g. node_modules with 50 000 files).
	node.Children = make([]*Node, 0, len(entries))

	// childrenWg tracks our direct subdirectory children so we can wait for
	// all their sizes to propagate before we propagate our own size upward.
	var childrenWg sync.WaitGroup

	for _, entry := range entries {
		if ctx.Err() != nil {
			break
		}

		entryPath := filepath.Join(node.Path, entry.Name())
		child := &Node{
			Name:  entry.Name(),
			Path:  entryPath,
			IsDir: entry.IsDir(),
		}

		// Never follow symlinks — avoids infinite cycles.
		if entry.Type()&fs.ModeSymlink != 0 {
			child.IsDir = false
			node.Children = append(node.Children, child)
			continue
		}

		if entry.IsDir() {
			node.Children = append(node.Children, child)
			childrenWg.Add(1)
			globalWg.Add(1)
			go scanDir(ctx, child, node, &childrenWg, sem, progressCh, globalWg)
		} else {
			info, err := entry.Info()
			if err != nil {
				node.Children = append(node.Children, child)
				continue
			}
			size := info.Size()
			child.SetSize(size)
			node.AddSize(size)
			sendProgress(ctx, progressCh, size)
			node.Children = append(node.Children, child)
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
