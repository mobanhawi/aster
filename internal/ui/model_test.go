package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

func nodeWithSize(name string, isDir bool, size int64, children ...*Node) *Node {
	n := &Node{Name: name, IsDir: isDir}
	n.SetSize(size)
	n.Children = children
	for _, c := range children {
		c.Parent = n
	}
	return n
}

func browsingModel(root *Node) Model {
	m := New(root.Name) // For root, Name is the full path
	m.root = root
	m.state = StateBrowsing
	m.width = 120
	m.height = 40
	return m
}

// ── Navigation tests ─────────────────────────────────────────────────────────

func TestNavigation(t *testing.T) {
	root := nodeWithSize("root", true, 1600,
		nodeWithSize("large", true, 1000),
		nodeWithSize("medium", true, 500),
		nodeWithSize("small.txt", false, 100),
	)

	testCases := []struct {
		name       string
		keys       []string
		wantCursor int
		wantDepth  int // len(stack)
	}{
		{
			name:       "GivenFirstItem_WhenMoveDown_ThenCursorIncremented",
			keys:       []string{"down"},
			wantCursor: 1,
			wantDepth:  0,
		},
		{
			name:       "GivenFirstItem_WhenMoveUp_ThenCursorStaysAtZero",
			keys:       []string{"up"},
			wantCursor: 0,
			wantDepth:  0,
		},
		{
			name:       "GivenDirSelected_WhenEnterPressed_ThenStackGrows",
			keys:       []string{"enter"},
			wantCursor: 0,
			wantDepth:  1,
		},
		{
			name:       "GivenInsideDir_WhenBackspacePressed_ThenStackShrinks",
			keys:       []string{"enter", "backspace"},
			wantCursor: 0,
			wantDepth:  0,
		},
		{
			name:       "GivenAtRoot_WhenBackspacePressed_ThenStackRemainsEmpty",
			keys:       []string{"backspace"},
			wantCursor: 0,
			wantDepth:  0,
		},
		{
			name:       "GivenMultipleMoves_WhenGPressed_ThenCursorJumpsToLast",
			keys:       []string{"G"},
			wantCursor: 2, // 3 children, index 2
			wantDepth:  0,
		},
		{
			name:       "GivenAtBottom_WhengPressed_ThenCursorJumpsToFirst",
			keys:       []string{"G", "g"},
			wantCursor: 0,
			wantDepth:  0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := browsingModel(root)

			var model tea.Model = m
			for _, key := range tc.keys {
				// Send only one message per key to avoid double-counting.
				var msg tea.KeyMsg
				switch key {
				case "enter":
					msg = tea.KeyMsg{Type: tea.KeyEnter}
				case "backspace":
					msg = tea.KeyMsg{Type: tea.KeyBackspace}
				case "up":
					msg = tea.KeyMsg{Type: tea.KeyUp}
				case "down":
					msg = tea.KeyMsg{Type: tea.KeyDown}
				default:
					msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
				}
				model, _ = model.Update(msg)
			}

			got := model.(Model)
			if got.cursor != tc.wantCursor {
				t.Errorf("cursor = %d, want %d", got.cursor, tc.wantCursor)
			}
			if len(got.stack) != tc.wantDepth {
				t.Errorf("stack depth = %d, want %d", len(got.stack), tc.wantDepth)
			}
		})
	}
}

// ── Sort tests ────────────────────────────────────────────────────────────────

func TestSortToggle(t *testing.T) {
	root := nodeWithSize("root", true, 1600,
		nodeWithSize("zebra.txt", false, 100),
		nodeWithSize("apple.txt", false, 500),
	)

	t.Run("GivenSortBySize_WhenSToggled_ThenSwitchesToSortByName", func(t *testing.T) {
		m := browsingModel(root)
		if m.sort != SortBySize {
			t.Fatalf("initial sort = %v, want SortBySize", m.sort)
		}

		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
		got := newModel.(Model)

		if got.sort != SortByName {
			t.Errorf("sort = %v, want SortByName", got.sort)
		}
		// First child should be "apple" (alphabetical)
		children := got.visibleChildren()
		if len(children) > 0 && children[0].Name != "apple.txt" {
			t.Errorf("first child = %q, want %q", children[0].Name, "apple.txt")
		}
	})

	t.Run("GivenSortByName_WhenSToggled_ThenSwitchesToSortBySize", func(t *testing.T) {
		m := browsingModel(root)
		m.sort = SortByName

		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
		got := newModel.(Model)

		if got.sort != SortBySize {
			t.Errorf("sort = %v, want SortBySize", got.sort)
		}
		// First child should be "apple.txt" (larger at 500)
		children := got.visibleChildren()
		if len(children) > 0 && children[0].Name != "apple.txt" {
			t.Errorf("first child after re-sort = %q, want %q", children[0].Name, "apple.txt")
		}
	})
}

// ── Delete confirm flow tests ─────────────────────────────────────────────────

func TestDeleteConfirmFlow(t *testing.T) {
	root := nodeWithSize("root", true, 600,
		nodeWithSize("fileA.txt", false, 500),
		nodeWithSize("fileB.txt", false, 100),
	)

	t.Run("GivenBrowsing_WhenDPressed_ThenStateChangesToConfirmDelete", func(t *testing.T) {
		m := browsingModel(root)

		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
		got := newModel.(Model)

		if got.state != StateConfirmDelete {
			t.Errorf("state = %v, want StateConfirmDelete", got.state)
		}
		if got.confirmPath == "" {
			t.Error("confirmPath should not be empty after 'd' pressed")
		}
	})

	t.Run("GivenConfirmDelete_WhenEscPressed_ThenReturnsToBrowsing", func(t *testing.T) {
		m := browsingModel(root)
		m.state = StateConfirmDelete
		m.confirmPath = "root/fileA.txt" // FullPath of fileA under root

		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		got := newModel.(Model)

		if got.state != StateBrowsing {
			t.Errorf("state = %v, want StateBrowsing", got.state)
		}
		if got.confirmPath != "" {
			t.Errorf("confirmPath = %q, want empty", got.confirmPath)
		}
	})

	t.Run("GivenConfirmDelete_WhenNPressed_ThenReturnsToBrowsing", func(t *testing.T) {
		m := browsingModel(root)
		m.state = StateConfirmDelete
		m.confirmPath = "root/fileA.txt"

		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
		got := newModel.(Model)

		if got.state != StateBrowsing {
			t.Errorf("state = %v, want StateBrowsing", got.state)
		}
	})
}

// ── Scroll window tests ───────────────────────────────────────────────────────

func TestScrollWindow(t *testing.T) {
	testCases := []struct {
		name      string
		cursor    int
		total     int
		height    int
		wantStart int
		wantEnd   int
	}{
		{
			name:      "GivenFewerItemsThanHeight_WhenScrolled_ThenFullRange",
			cursor:    0,
			total:     5,
			height:    10,
			wantStart: 0,
			wantEnd:   5,
		},
		{
			name:      "GivenCursorAtTop_WhenScrolled_ThenStartsAtZero",
			cursor:    0,
			total:     100,
			height:    10,
			wantStart: 0,
			wantEnd:   10,
		},
		{
			name:      "GivenCursorAtBottom_WhenScrolled_ThenEndsAtTotal",
			cursor:    99,
			total:     100,
			height:    10,
			wantStart: 90,
			wantEnd:   100,
		},
		{
			name:      "GivenCursorInMiddle_WhenScrolled_ThenCentresWindow",
			cursor:    50,
			total:     100,
			height:    10,
			wantStart: 45,
			wantEnd:   55,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			start, end := scrollWindow(tc.cursor, tc.total, tc.height)
			if start != tc.wantStart {
				t.Errorf("start = %d, want %d", start, tc.wantStart)
			}
			if end != tc.wantEnd {
				t.Errorf("end = %d, want %d", end, tc.wantEnd)
			}
		})
	}
}

// ── Truncate tests ────────────────────────────────────────────────────────────

func TestTruncate(t *testing.T) {
	testCases := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "GivenShortString_WhenTruncated_ThenReturnedUnchanged",
			input:  "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "GivenExactLength_WhenTruncated_ThenReturnedUnchanged",
			input:  "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			// maxLen=7: take first 6 runes (7-1) then append ellipsis => "hello …"
			name:   "GivenLongString_WhenTruncated_ThenEllipsisAppended",
			input:  "hello world",
			maxLen: 7,
			want:   "hello …",
		},
		{
			name:   "GivenZeroMaxLen_WhenTruncated_ThenReturnsEmpty",
			input:  "hello",
			maxLen: 0,
			want:   "",
		},
		{
			name:   "GivenUnicodeString_WhenTruncated_ThenRespectRuneBoundaries",
			input:  "αβγδε",
			maxLen: 3,
			want:   "αβ…",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := truncate(tc.input, tc.maxLen)
			if got != tc.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tc.input, tc.maxLen, got, tc.want)
			}
		})
	}
}

// ── ClampCursor tests ─────────────────────────────────────────────────────────

func TestClampCursor(t *testing.T) {
	root := nodeWithSize("root", true, 300,
		nodeWithSize("a.txt", false, 100),
		nodeWithSize("b.txt", false, 100),
	)

	t.Run("GivenCursorBeyondList_WhenClamped_ThenCursorMovesToLast", func(t *testing.T) {
		m := browsingModel(root)
		m.cursor = 99
		m.clampCursor()
		if m.cursor != 1 {
			t.Errorf("cursor = %d, want 1", m.cursor)
		}
	})

	t.Run("GivenCursorNegative_WhenClamped_ThenCursorMovesToZero", func(t *testing.T) {
		m := browsingModel(root)
		m.cursor = -5
		m.clampCursor()
		if m.cursor != 0 {
			t.Errorf("cursor = %d, want 0", m.cursor)
		}
	})

	t.Run("GivenEmptyChildren_WhenClamped_ThenCursorStaysZero", func(t *testing.T) {
		empty := nodeWithSize("empty", true, 0)
		m := browsingModel(empty)
		m.cursor = 5
		m.clampCursor()
		if m.cursor != 0 {
			t.Errorf("cursor = %d, want 0", m.cursor)
		}
	})
}

// ── ScanDone state transition ─────────────────────────────────────────────────

func TestScanDoneTransition(t *testing.T) {
	t.Run("GivenSuccessfulScan_WhenScanDoneReceived_ThenSwitchesToBrowsing", func(t *testing.T) {
		m := New("/tmp")
		root := nodeWithSize("root", true, 100)

		newModel, _ := m.Update(scanDoneMsg{root: root})
		got := newModel.(Model)

		if got.state != StateBrowsing {
			t.Errorf("state = %v, want StateBrowsing", got.state)
		}
		if got.root != root {
			t.Error("root node not set correctly after scanDoneMsg")
		}
	})

	t.Run("GivenScanError_WhenScanDoneReceived_ThenSwitchesToError", func(t *testing.T) {
		m := New("/tmp")
		scanErr := errScanFailed("permission denied")

		newModel, _ := m.Update(scanDoneMsg{err: scanErr})
		got := newModel.(Model)

		if got.state != StateError {
			t.Errorf("state = %v, want StateError", got.state)
		}
		if got.scanErr == nil {
			t.Error("scanErr should be set after error scanDoneMsg")
		}
	})
}

// errScanFailed is a test helper error type.
type errScanFailed string

func (e errScanFailed) Error() string { return string(e) }
