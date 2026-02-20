package scanner

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// Scan walks the directory tree rooted at root concurrently.
// progressCh (optional) receives byte counts as files are encountered.
// The channel is NOT closed by Scan; the caller should close it after use.
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
			select {
			case progressCh <- info.Size():
			case <-ctx.Done():
			}
		}
		return rootNode, nil
	}

	// Limit concurrent os.ReadDir calls, NOT the overall goroutine tree.
	// Holding the semaphore only during I/O prevents the classic deadlock where
	// goroutines hold a slot while blocking in sync.WaitGroup.Wait() for
	// children that are also waiting for a slot.
	maxConcurrent := runtime.NumCPU() * 2
	if maxConcurrent < 4 {
		maxConcurrent = 4
	}
	sem := make(chan struct{}, maxConcurrent)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanDir(ctx, rootNode, sem, progressCh)
	}()
	wg.Wait()

	return rootNode, nil
}

// scanDir recursively scans a directory. The semaphore is acquired only for
// the os.ReadDir syscall, then immediately released before spawning children.
// This avoids the deadlock where a goroutine holds a semaphore slot while
// waiting for child goroutines that need the same slots.
func scanDir(ctx context.Context, node *Node, sem chan struct{}, progressCh chan<- int64) {
	// Acquire slot only for the directory read I/O.
	select {
	case sem <- struct{}{}:
	case <-ctx.Done():
		return
	}
	entries, err := os.ReadDir(node.Path)
	<-sem // Release immediately — not held during child goroutines.

	if err != nil {
		node.Err = err
		return
	}

	var (
		mu      sync.Mutex
		childWg sync.WaitGroup
	)

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

		// Don't follow symlinks — avoids cycles.
		if entry.Type()&fs.ModeSymlink != 0 {
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
			go func(c *Node) {
				defer childWg.Done()
				scanDir(ctx, c, sem, progressCh)
				node.AddSize(c.Size())
			}(child)
		} else {
			info, err := entry.Info()
			if err != nil {
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
