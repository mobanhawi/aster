package scanner

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// Result is returned by Scan when scanning completes.
type Result struct {
	Root *Node
	Err  error
}

// Scan walks the directory tree rooted at root concurrently.
// It returns the root Node and a progress channel that emits cumulative bytes scanned.
// The progress channel is closed when scanning is complete.
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
		if progressCh != nil {
			progressCh <- info.Size()
		}
		return rootNode, nil
	}

	workers := runtime.NumCPU() * 2
	if workers < 4 {
		workers = 4
	}

	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup

	scanDir(ctx, rootNode, sem, &wg, progressCh)
	wg.Wait()

	return rootNode, nil
}

func scanDir(ctx context.Context, node *Node, sem chan struct{}, wg *sync.WaitGroup, progressCh chan<- int64) {
	entries, err := os.ReadDir(node.Path)
	if err != nil {
		node.Err = err
		return
	}

	var mu sync.Mutex
	var childWg sync.WaitGroup

	for _, entry := range entries {
		if ctx.Err() != nil {
			return
		}

		entryPath := filepath.Join(node.Path, entry.Name())

		child := &Node{
			Name:  entry.Name(),
			Path:  entryPath,
			IsDir: entry.IsDir(),
		}

		// Handle symlinks: don't follow them (avoid cycles).
		// entry.Type() is available on fs.DirEntry without calling Info().
		if entry.Type()&fs.ModeSymlink != 0 {
			// Treat symlink as zero-size leaf (don't follow).
			child.IsDir = false
			mu.Lock()
			node.Children = append(node.Children, child)
			mu.Unlock()
			continue
		}

		if entry.IsDir() {
			mu.Lock()
			node.Children = append(node.Children, child)
			mu.Unlock()

			childWg.Add(1)
			wg.Add(1)

			go func(c *Node) {
				defer childWg.Done()
				defer wg.Done()

				// Acquire semaphore slot
				select {
				case sem <- struct{}{}:
				case <-ctx.Done():
					return
				}
				scanDir(ctx, c, sem, wg, progressCh)
				<-sem

				// Bubble size up to parent
				node.AddSize(c.Size())
			}(child)
		} else {
			info, err := entry.Info()
			if err != nil {
				// Skip unreadable entries
				continue
			}
			size := info.Size()
			child.SetSize(size)
			node.AddSize(size)

			if progressCh != nil {
				select {
				case progressCh <- size:
				case <-ctx.Done():
					return
				}
			}

			mu.Lock()
			node.Children = append(node.Children, child)
			mu.Unlock()
		}
	}

	childWg.Wait()
}
