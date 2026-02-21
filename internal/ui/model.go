package ui

import (
	"context"
	"strings"
	"sync/atomic"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	humanize "github.com/dustin/go-humanize"
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

// sortModeToInt8 converts a SortMode to the int8 stored in Node.SortedMode.
func sortModeToInt8(m SortMode) int8 {
	return int8(m) // #nosec G115 -- SortMode values are small iota constants (0, 1)
}

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

	// sortGen is incremented each time the sort mode changes so that nodes
	// detect staleness in O(1) instead of walking the entire tree.
	sortGen uint64

	// Scan state
	state    AppState
	rootPath string
	absRoot  string // resolved once — avoids filepath.Abs on every View()
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

	// Purgeable space state
	purgeableSpace  int64
	purgeableReady  bool
	purgeableString string

	// Render caches — recomputed only when their inputs change.
	cachedDivider      string // "─" × width
	cachedDividerWidth int
	cachedHints        string // key-hint footer (static after init)
	cachedHintsWidth   int
	// cachedStatus caches the formatted humanize string for the status bar.
	cachedStatusSize  int64
	cachedStatusHuman string
}

// New constructs a fresh model targeting the given root path.
func New(rootPath string) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = styleScanning

	var scanned atomic.Int64
	return Model{
		rootPath:     rootPath,
		absRoot:      rootPath, // refined in startScan after Abs resolves
		state:        StateScanning,
		sp:           sp,
		scannedBytes: &scanned,
		sortGen:      1, // start at 1 so zero-value nodes are always stale
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	// Create a buffered progress channel; the scanner will send byte counts.
	m.progressCh = make(chan int64, 4096)
	return tea.Batch(
		m.sp.Tick,
		startScan(m.rootPath, m.progressCh, m.scannedBytes),
		fetchPurgeable(m.rootPath),
	)
}

// purgeableSpaceMsg is sent when the background purgeable space fetch completes.
type purgeableSpaceMsg struct {
	space int64
	str   string
}

// fetchPurgeable computes the volume's purgeable space asynchronously.
func fetchPurgeable(path string) tea.Cmd {
	return func() tea.Msg {
		space := scanner.GetPurgeableSpace(path)
		if space < 0 {
			space = 0
		}
		return purgeableSpaceMsg{
			space: space,
			str:   humanize.Bytes(uint64(space)),
		}
	}
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
// The sortGen/SortedMode fields are NOT updated here — the caller (visibleChildren)
// stamps the generation after sorting to keep the contract simple.
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
// them lazily on first access using the generation counter so that a sort
// toggle is O(1) (just bumps sortGen) rather than O(N) (tree walk).
func (m *Model) visibleChildren() []*Node {
	d := m.currentDir()
	if d == nil {
		return nil
	}
	modeInt := sortModeToInt8(m.sort)
	if !d.IsSorted(m.sortGen, modeInt) {
		sortNode(d, m.sort)
		d.MarkSorted(m.sortGen, modeInt)
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

// divider returns a cached "─" × m.width string, refreshing only when width
// changes to avoid a strings.Repeat allocation on every frame.
func (m *Model) divider() string {
	if m.cachedDividerWidth != m.width {
		m.cachedDivider = styleDivider.Render(strings.Repeat("─", m.width))
		m.cachedDividerWidth = m.width
	}
	return m.cachedDivider
}

// keyHints returns the cached footer key-hint string, rebuilding only when
// the terminal width changes (which is rare).
func (m *Model) keyHints() string {
	if m.cachedHintsWidth != m.width {
		k := func(key, desc string) string {
			return styleKey.Render(key) + " " + desc + "  "
		}
		raw := " " +
			k("↑↓/jk", "move") +
			k("→/enter", "enter") +
			k("←/bsp", "back") +
			k("o", "open") +
			k("r", "reveal") +
			k("d", "delete") +
			k("s", "sort") +
			k("q", "quit")
		m.cachedHints = styleFooter.Width(m.width).Render(raw)
		m.cachedHintsWidth = m.width
	}
	return m.cachedHints
}

// humanSize returns a cached humanize.Bytes string for sz, refreshing only
// when sz changes. This avoids the humanize allocation on every render frame
// for the status-bar total-size display.
func (m *Model) humanSize(sz int64) string {
	if sz != m.cachedStatusSize || m.cachedStatusHuman == "" {
		m.cachedStatusSize = sz
		if sz < 0 {
			sz = 0
		}
		m.cachedStatusHuman = humanize.Bytes(uint64(sz)) // #nosec G115 -- sz is file size, non-negative
	}
	return m.cachedStatusHuman
}

// barCache is a per-render cache of rendered bar strings, keyed by (barLen,
// barMaxW) to avoid re-rendering identical bars (common at list extremes where
// many files share the same relative size).
//
// It is stored on the stack as a map built once per viewBrowse call and
// discarded after. It is NOT a package-level cache because bar rendering
// depends on both rank and width, which can change across frames.

// Sorted flag for root after init.
func (m *Model) markRootSorted() {
	if m.root != nil {
		m.root.MarkSorted(m.sortGen, sortModeToInt8(m.sort))
	}
}

// itoa is a tiny allocation-free int→string for small non-negative values.
// For large N it falls back to the stdlib formatter.
func itoa(n int) string {
	if n < len(itoaTable) {
		return itoaTable[n]
	}
	// Fallback: build manually to avoid fmt import dependency here.
	return humanize.Comma(int64(n))
}

// itoaTable holds pre-formatted strings for the most common item counts.
// Directories rarely have > 10k direct children, so 10 000 entries covers > 99 % of cases.
var itoaTable = func() []string {
	t := make([]string, 10001)
	for i := range t {
		// Build without fmt to keep this self-contained.
		t[i] = humanize.Comma(int64(i))
	}
	return t
}()
