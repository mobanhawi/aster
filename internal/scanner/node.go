package scanner

import (
	"cmp"
	"slices"
	"sync/atomic"
)

// Node represents a file or directory in the scanned tree.
type Node struct {
	Name     string
	Path     string
	IsDir    bool
	size     atomic.Int64 // atomic for concurrent updates
	Children []*Node
	Err      error // non-nil if scan failed (e.g. permission denied)

	// Sorted is set to true once Children have been sorted. Reset to false when
	// a new sort order is requested, enabling lazy per-directory sorting.
	// Accessed only from the single-threaded UI goroutine after scan completes.
	Sorted bool
}

// Size returns the total size in bytes (recursive for dirs).
func (n *Node) Size() int64 {
	return n.size.Load()
}

// AddSize atomically adds bytes to this node's size counter.
func (n *Node) AddSize(bytes int64) {
	n.size.Add(bytes)
}

// SetSize sets the size directly (non-concurrent use only).
func (n *Node) SetSize(bytes int64) {
	n.size.Store(bytes)
}

// SortBySize sorts children by size descending (largest first).
func (n *Node) SortBySize() {
	slices.SortFunc(n.Children, func(a, b *Node) int {
		return cmp.Compare(b.Size(), a.Size())
	})
	n.Sorted = true
}

// SortByName sorts children alphabetically by name.
func (n *Node) SortByName() {
	slices.SortFunc(n.Children, func(a, b *Node) int {
		return cmp.Compare(a.Name, b.Name)
	})
	n.Sorted = true
}

// ResetSorted recursively marks n and all descendants as unsorted so that the
// next navigation visit will re-sort on demand.
func (n *Node) ResetSorted() {
	if n == nil {
		return
	}
	n.Sorted = false
	for _, child := range n.Children {
		if child.IsDir {
			child.ResetSorted()
		}
	}
}
