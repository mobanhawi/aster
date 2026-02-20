package ui

import (
	"context"
	"sync/atomic"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mobanhawi/aster/internal/scanner"
)

// SortMode controls how children are ordered.
type SortMode int

const (
	// SortBySize sorts items by descending size.
	SortBySize SortMode = iota
	// SortByName sorts items alphabetically.
	SortByName
)

// scanDoneMsg is sent when scanning completes.
type scanDoneMsg struct {
	root *Node
	err  error
}

// progressTickMsg is sent periodically to update the scanned-bytes counter
// displayed while scanning is in progress.
type progressTickMsg struct{ scanned int64 }

// Node is a local alias for the scanner node.
type Node = scanner.Node

// AppState controls what the model is showing.
type AppState int

const (
	// StateScanning is the initial scanning progress view.
	StateScanning AppState = iota
	// StateBrowsing is the interactive file browser.
	StateBrowsing
	// StateConfirmDelete shows the deletion prompt overlay.
	StateConfirmDelete
	// StateError displays any unrecoverable errors.
	StateError
)

// Model is the Bubble Tea application model.
type Model struct {
	// Navigation state
	root   *Node
	stack  []*Node // breadcrumb stack; current dir = stack[len-1]
	cursor int
	sort   SortMode

	// Scan state
	state    AppState
	rootPath string
	scanErr  error

	// UI dimensions
	width  int
	height int

	// Widgets
	sp spinner.Model

	// Confirm-delete state
	confirmPath string

	// Live scan progress (updated from progressCh via atomic).
	scannedBytes *atomic.Int64 // pointer so Model copies share the counter
	progressCh   chan int64

	// Total disk size hint from Statfs (0 if unavailable).
	diskTotalBytes int64
}

// New constructs a fresh model targeting the given root path.
func New(rootPath string) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = styleScanning

	var scanned atomic.Int64
	return Model{
		rootPath:       rootPath,
		state:          StateScanning,
		sp:             sp,
		scannedBytes:   &scanned,
		diskTotalBytes: diskTotal(rootPath),
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	// Create a buffered progress channel; the scanner will send byte counts.
	m.progressCh = make(chan int64, 4096)
	return tea.Batch(
		m.sp.Tick,
		startScan(m.rootPath, m.progressCh, m.scannedBytes),
	)
}

// startScan launches the concurrent scanner in a goroutine that also drains
// progressCh into scanned (atomic) so the view can display live byte counts.
func startScan(root string, progressCh chan int64, scanned *atomic.Int64) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Drain the progress channel in a separate goroutine.
		var drainDone = make(chan struct{})
		go func() {
			defer close(drainDone)
			for b := range progressCh {
				scanned.Add(b)
			}
		}()

		node, err := scanner.Scan(ctx, root, progressCh)
		close(progressCh)
		<-drainDone // wait for all progress bytes to land

		if err != nil {
			return scanDoneMsg{err: err}
		}

		// Sort only the root level eagerly; all other dirs sort lazily on
		// first navigation. This avoids a multi-second O(N log N) pause for
		// large trees before the UI becomes interactive.
		sortNode(node, SortBySize)

		return scanDoneMsg{root: node}
	}
}

// sortNode sorts a single node's children (not recursive).
// The Node.Sorted flag is set so visibleChildren knows it is already sorted.
func sortNode(n *Node, mode SortMode) {
	if n == nil {
		return
	}
	switch mode {
	case SortBySize:
		n.SortBySize()
	case SortByName:
		n.SortByName()
	}
}

// currentDir returns the directory currently being browsed.
func (m *Model) currentDir() *Node {
	if len(m.stack) == 0 {
		return m.root
	}
	return m.stack[len(m.stack)-1]
}

// visibleChildren returns the sorted children of the current dir, sorting
// them lazily on first access so the whole tree is never sorted at once.
func (m *Model) visibleChildren() []*Node {
	d := m.currentDir()
	if d == nil {
		return nil
	}
	if !d.Sorted {
		sortNode(d, m.sort)
	}
	return d.Children
}

// clampCursor ensures the cursor is within bounds.
func (m *Model) clampCursor() {
	n := len(m.visibleChildren())
	if n == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= n {
		m.cursor = n - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// selected returns the currently highlighted node (may be nil).
func (m *Model) selected() *Node {
	children := m.visibleChildren()
	if len(children) == 0 || m.cursor >= len(children) {
		return nil
	}
	return children[m.cursor]
}
