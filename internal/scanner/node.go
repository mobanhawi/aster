package scanner

import (
	"cmp"
	"os"
	"slices"
	"strings"
	"sync/atomic"
)

// Node represents a file or directory in the scanned tree.
// Optimized for memory to handle millions of files (terabytes of data).
type Node struct {
	// Parent allows path reconstruction without storing full path strings.
	// Net memory saving is ~100 bytes per node on average for deep trees.
	Parent *Node
	// Children stores sub-nodes. Nil for non-directories.
	Children []*Node

	// Name is just the file/dir name (e.g. "photo.jpg"), not the full path.
	// For the root node, this is the full starting path.
	Name string

	// size is atomic to support concurrent updates during scanning.
	size atomic.Int64

	// sortGen tracks the sort-mode generation (O(1) staleness check).
	sortGen uint64

	// Err stores any error encountered during scan of this node.
	Err error

	// SortedMode tracks the last SortMode used (e.g. size vs name).
	SortedMode int8

	// IsDir marks if this node can have children.
	IsDir bool
}

// FullPath reconstructs the absolute path by walking up to the root.
// This is slower than storing the path, but saves massive amounts of memory.
// It's called only on user interaction (open, delete, reveal), not in hot loops.
func (n *Node) FullPath() string {
	if n == nil {
		return ""
	}
	if n.Parent == nil {
		return n.Name // Root node stores its full path in Name
	}

	// Calculate total length to allocate once.
	var parts []string
	curr := n
	length := 0
	for curr != nil {
		parts = append(parts, curr.Name)
		length += len(curr.Name) + 1
		curr = curr.Parent
	}

	// Build string backwards.
	var sb strings.Builder
	sb.Grow(length)
	sep := string(os.PathSeparator)
	for i := len(parts) - 1; i >= 0; i-- {
		sb.WriteString(parts[i])
		if i > 0 && !strings.HasSuffix(parts[i], sep) {
			sb.WriteString(sep)
		}
	}
	res := sb.String()
	// Clean up double separators if root ended with one
	return strings.ReplaceAll(res, sep+sep, sep)
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

// IsSorted reports whether this node's children are already sorted.
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

// ResetSorted is kept for backwards compatibility with tests.
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
