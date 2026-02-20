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

	// sortGen is the sort-generation at which this node's Children were last
	// sorted. When the global sort generation advances (e.g. on a sort-mode
	// toggle), sortGen falls behind and visibleChildren re-sorts lazily on
	// first access. This replaces the previous O(N) ResetSorted recursive
	// walk — advancing the generation counter is O(1).
	sortGen uint64

	// SortedMode is the SortMode that was used for the last sort, so we can
	// detect a mode change even within the same generation.
	SortedMode int8 // -1 = never sorted
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

// IsSorted reports whether this node's children are already sorted for the
// given generation and sort mode.
func (n *Node) IsSorted(gen uint64, mode int8) bool {
	return n.sortGen == gen && n.SortedMode == mode
}

// MarkSorted records that children are sorted for the given generation/mode.
func (n *Node) MarkSorted(gen uint64, mode int8) {
	n.sortGen = gen
	n.SortedMode = mode
}

// SortBySize sorts children by size descending (largest first).
func (n *Node) SortBySize() {
	slices.SortFunc(n.Children, func(a, b *Node) int {
		return cmp.Compare(b.Size(), a.Size())
	})
}

// SortByName sorts children alphabetically by name.
func (n *Node) SortByName() {
	slices.SortFunc(n.Children, func(a, b *Node) int {
		return cmp.Compare(a.Name, b.Name)
	})
}

// ResetSorted is kept for backwards compatibility with tests; it is no longer
// called on the hot path. The generation-counter mechanism in the Model
// replaces the O(N) recursive walk this used to perform.
func (n *Node) ResetSorted() {
	if n == nil {
		return
	}
	n.sortGen = 0
	for _, child := range n.Children {
		if child.IsDir {
			child.ResetSorted()
		}
	}
}
