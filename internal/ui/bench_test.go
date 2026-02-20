package ui

import (
	"testing"
)

// BenchmarkViewBrowse measures the cost of a full render frame in browsing mode.
// This is the hottest path — it runs on every key press and every terminal event.
func BenchmarkViewBrowse(b *testing.B) {
	// Build a root with 500 children to simulate a real browse session.
	root := nodeWithSize("root", true, 0)
	root.Children = make([]*Node, 500)
	for i := range 500 {
		child := nodeWithSize("file_"+itoa(i)+".bin", false, int64(i*1024))
		root.Children[i] = child
		root.AddSize(int64(i * 1024))
	}

	m := browsingModel(root)
	m.cursor = 250

	b.ResetTimer()
	for range b.N {
		_ = m.View()
	}
}

// BenchmarkSortToggle measures the O(1) sort-gen bump vs the previous O(N)
// recursive ResetSorted walk.
func BenchmarkSortToggle(b *testing.B) {
	// 10 000 nodes to make the difference obvious.
	root := nodeWithSize("root", true, 0)
	root.Children = make([]*Node, 10_000)
	for i := range 10_000 {
		root.Children[i] = nodeWithSize("d"+itoa(i), true, int64(i))
	}

	m := browsingModel(root)
	b.ResetTimer()
	for range b.N {
		m.handleSortToggle()
	}
}

// BenchmarkVisibleChildren measures lazy sort amortised cost.
func BenchmarkVisibleChildren(b *testing.B) {
	root := nodeWithSize("root", true, 0)
	root.Children = make([]*Node, 1_000)
	for i := range 1_000 {
		root.Children[i] = nodeWithSize("f"+itoa(i), false, int64(i))
	}

	m := browsingModel(root)
	b.ResetTimer()
	for i := range b.N {
		if i%100 == 0 {
			// Simulate a sort toggle every 100 frames.
			m.handleSortToggle()
		}
		_ = m.visibleChildren()
	}
}
