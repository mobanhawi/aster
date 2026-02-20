package scanner

import (
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
	sortNodes(n.Children, func(a, b *Node) bool {
		return a.Size() > b.Size()
	})
}

// SortByName sorts children alphabetically by name.
func (n *Node) SortByName() {
	sortNodes(n.Children, func(a, b *Node) bool {
		return a.Name < b.Name
	})
}

// sortNodes is an in-place insertion sort for small-to-medium slices.
func sortNodes(nodes []*Node, less func(a, b *Node) bool) {
	for i := 1; i < len(nodes); i++ {
		key := nodes[i]
		j := i - 1
		for j >= 0 && less(key, nodes[j]) {
			nodes[j+1] = nodes[j]
			j--
		}
		nodes[j+1] = key
	}
}
