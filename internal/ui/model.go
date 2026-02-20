package ui

import (
	"context"

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
}

// New constructs a fresh model targeting the given root path.
func New(rootPath string) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = styleScanning

	return Model{
		rootPath: rootPath,
		state:    StateScanning,
		sp:       sp,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.sp.Tick,
		startScan(m.rootPath),
	)
}

// startScan launches the concurrent scanner and delivers the result as a Cmd.
func startScan(root string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		// We don't use the progress channel in the model for now
		// (the spinner suffices); pass nil.
		node, err := scanner.Scan(ctx, root, nil)
		if err != nil {
			return scanDoneMsg{err: err}
		}
		// Sort by size immediately after scan
		sortTree(node, SortBySize)
		return scanDoneMsg{root: node}
	}
}

// sortTree recursively sorts a node's children by the given SortMode.
func sortTree(n *Node, mode SortMode) {
	if n == nil {
		return
	}
	switch mode {
	case SortBySize:
		n.SortBySize()
	case SortByName:
		n.SortByName()
	}
	for _, child := range n.Children {
		if child.IsDir {
			sortTree(child, mode)
		}
	}
}

// currentDir returns the directory currently being browsed.
func (m *Model) currentDir() *Node {
	if len(m.stack) == 0 {
		return m.root
	}
	return m.stack[len(m.stack)-1]
}

// visibleChildren returns the sortable children of the current dir.
func (m *Model) visibleChildren() []*Node {
	d := m.currentDir()
	if d == nil {
		return nil
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
