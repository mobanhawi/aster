package scanner

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

// Scan walks the directory tree rooted at root concurrently using a semaphore-
// limited goroutine-per-directory model.
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
		Name:  absRoot,
		IsDir: info.IsDir(),
	}

	if !info.IsDir() {
		rootNode.SetSize(info.Size())
		sendProgress(ctx, progressCh, info.Size())
		return rootNode, nil
	}

	numWorkers := runtime.NumCPU() * 32
	if numWorkers < 256 {
		numWorkers = 256
	}
	sem := make(chan struct{}, numWorkers)

	var globalWg sync.WaitGroup
	globalWg.Add(1)
	go scanDir(ctx, rootNode, absRoot, nil, sem, progressCh, &globalWg)
	globalWg.Wait()

	return rootNode, nil
}

const (
	// readDirBatchSize controls how many entries we read from disk at once.
	// Processing in batches keeps peak memory usage low for massive directories
	// (e.g. 1M+ files) and improves UI responsiveness.
	readDirBatchSize = 1024
)

// scanDir reads a single directory, processes its file children inline, and
// spawns a new goroutine (bounded by sem) for each subdirectory child.
func scanDir(
	ctx context.Context,
	node *Node,
	currentPath string,
	parentWg *sync.WaitGroup,
	sem chan struct{},
	progressCh chan<- int64,
	globalWg *sync.WaitGroup,
) {
	var localChildrenWg sync.WaitGroup

	defer func() {
		localChildrenWg.Wait()
		if node.Parent != nil {
			node.Parent.AddSize(node.Size())
		}
		if parentWg != nil {
			parentWg.Done()
		}
		globalWg.Done()
	}()

	if ctx.Err() != nil {
		return
	}

	// Bypassing os.ReadDir to:
	// 1. Avoid the mandatory alphabetical sort (we sort lazily in UI).
	// 2. Process in chunks to cap peak memory for massive directories.
	sem <- struct{}{}
	// #nosec G304,G703 -- currentPath is a directory being scanned, sanitized by filepath.Abs in Scan()
	f, err := os.Open(currentPath)
	if err != nil {
		<-sem
		node.Err = err
		return
	}

	sep := string(os.PathSeparator)
	dirPrefix := currentPath
	if !strings.HasSuffix(dirPrefix, sep) {
		dirPrefix += sep
	}

	for ctx.Err() == nil {
		// Read a batch of entries.
		// Using f.ReadDir instead of os.ReadDir bypasses the stdlib's sorting.
		entries, err := f.ReadDir(readDirBatchSize)
		if err != nil {
			if err != io.EOF {
				node.Err = err
			}
			break
		}
		if len(entries) == 0 {
			break
		}

		processBatch(ctx, node, entries, dirPrefix, sem, &localChildrenWg, progressCh, globalWg)
	}

	if cerr := f.Close(); cerr != nil && node.Err == nil {
		node.Err = cerr
	}
	<-sem
}

// processBatch handles logical processing for a chunk of directory entries,
// reducing the cyclomatic complexity of scanDir.
func processBatch(
	ctx context.Context,
	node *Node,
	entries []fs.DirEntry,
	dirPrefix string,
	sem chan struct{},
	localChildrenWg *sync.WaitGroup,
	progressCh chan<- int64,
	globalWg *sync.WaitGroup,
) {
	// Pre-grow children slice to minimize reallocs.
	needed := len(node.Children) + len(entries)
	if cap(node.Children) < needed {
		newCap := cap(node.Children) * 2
		if newCap < needed {
			newCap = needed
		}
		newChildren := make([]*Node, len(node.Children), newCap)
		copy(newChildren, node.Children)
		node.Children = newChildren
	}

	startChildIdx := len(node.Children)

	// Append all children to node first
	for _, entry := range entries {
		child := &Node{
			Name:   entry.Name(),
			Parent: node,
			IsDir:  entry.IsDir(),
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			child.IsDir = false
		}
		node.Children = append(node.Children, child)
	}

	var wg sync.WaitGroup
	var totalSize atomic.Int64

	numChunks := 8
	if len(entries) < 32 {
		numChunks = 1
	}

	chunkSize := (len(entries) + numChunks - 1) / numChunks

	for i := 0; i < numChunks; i++ {
		start := i * chunkSize
		if start >= len(entries) {
			break
		}
		end := start + chunkSize
		if end > len(entries) {
			end = len(entries)
		}

		wg.Add(1)
		go func(s, e int) {
			defer wg.Done()
			var localSize int64

			for j := s; j < e; j++ {
				entry := entries[j]
				child := node.Children[startChildIdx+j]

				if entry.Type()&fs.ModeSymlink != 0 {
					continue
				}

				if entry.IsDir() {
					localChildrenWg.Add(1)
					globalWg.Add(1)
					childPath := dirPrefix + entry.Name()
					go scanDir(ctx, child, childPath, localChildrenWg, sem, progressCh, globalWg)
				} else {
					info, err := entry.Info()
					if err == nil {
						sz := info.Size()
						child.SetSize(sz)
						localSize += sz
					}
				}
			}
			if localSize > 0 {
				totalSize.Add(localSize)
			}
		}(start, end)
	}
	wg.Wait()

	batchFilesSize := totalSize.Load()
	if batchFilesSize > 0 {
		node.AddSize(batchFilesSize)
		sendProgress(ctx, progressCh, batchFilesSize)
	}
}

func sendProgress(ctx context.Context, ch chan<- int64, sz int64) {
	if ch == nil || sz == 0 || ctx.Err() != nil {
		return
	}
	select {
	case ch <- sz:
	default:
	}
}
