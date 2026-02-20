package ui

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.sp, cmd = m.sp.Update(msg)
		return m, cmd

	case scanDoneMsg:
		if msg.err != nil {
			m.state = StateError
			m.scanErr = msg.err
			return m, nil
		}
		m.root = msg.root
		m.state = StateBrowsing
		m.cursor = 0
		m.stack = nil
		// Cache the resolved absolute path so breadcrumb() avoids calling
		// filepath.Abs on every render frame.
		if msg.root != nil {
			m.absRoot = msg.root.Path
			// Mark the root as already sorted (startScan sorted it eagerly).
			m.markRootSorted()
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case StateScanning:
		return m.handleKeyScanning(msg)
	case StateConfirmDelete:
		return m.handleKeyConfirmDelete(msg)
	case StateBrowsing:
		return m.handleKeyBrowsing(msg)
	}
	return m, nil
}

func (m Model) handleKeyScanning(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" || msg.String() == "q" {
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleKeyConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "d", "y", "enter":
		err := trashItem(m.confirmPath)
		if err == nil {
			// Remove from parent's children list
			parent := m.currentDir()
			removedSize := int64(0)
			for i, c := range parent.Children {
				if c.Path == m.confirmPath {
					removedSize = c.Size()
					parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
					break
				}
			}
			// Deduct size up the stack
			for _, anc := range m.stack {
				anc.AddSize(-removedSize)
			}
			if m.root != nil {
				m.root.AddSize(-removedSize)
			}
			m.clampCursor()
		}
		m.state = StateBrowsing
		m.confirmPath = ""
	case "esc", "n", "q":
		m.state = StateBrowsing
		m.confirmPath = ""
	}
	return m, nil
}

func (m Model) handleKeyBrowsing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Intercept and handle basic navigation
	switch key {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "down", "j":
		if m.cursor < len(m.visibleChildren())-1 {
			m.cursor++
		}
		return m, nil
	case "right", "enter", "l":
		return m.handleNavRight()
	case "left", "backspace", "h":
		if len(m.stack) > 0 {
			m.stack = m.stack[:len(m.stack)-1]
			m.clampCursor()
		}
		return m, nil
	}

	// Dispatch commands to secondary handler
	return m.handleKeyBrowsingActions(key)
}

func (m Model) handleKeyBrowsingActions(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "s":
		m.handleSortToggle()
	case "o":
		if err := m.handleOpen(); err != nil {
			m.scanErr = err
			m.state = StateError
		}
	case "r":
		if err := m.handleReveal(); err != nil {
			m.scanErr = err
			m.state = StateError
		}
	case "d":
		sel := m.selected()
		if sel != nil {
			m.state = StateConfirmDelete
			m.confirmPath = sel.Path
		}
	case "g", "home":
		m.cursor = 0
	case "G", "end":
		if n := len(m.visibleChildren()); n > 0 {
			m.cursor = n - 1
		}
	}
	return m, nil
}

func (m *Model) handleNavRight() (tea.Model, tea.Cmd) {
	sel := m.selected()
	if sel != nil && sel.IsDir {
		m.stack = append(m.stack, sel)
		m.cursor = 0
	}
	return *m, nil
}

func (m *Model) handleSortToggle() {
	if m.sort == SortBySize {
		m.sort = SortByName
	} else {
		m.sort = SortBySize
	}
	// Advance the sort generation: each Node caches the generation at which it
	// was last sorted. A mismatch triggers a lazy re-sort on first access —
	// O(1) here instead of an O(N) recursive flag walk across the whole tree.
	m.sortGen++
	m.cursor = 0
}

func (m *Model) handleOpen() error {
	sel := m.selected()
	if sel != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return openPath(ctx, sel.Path)
	}
	return nil
}

func (m *Model) handleReveal() error {
	sel := m.selected()
	if sel != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return revealPath(ctx, sel.Path)
	}
	return nil
}

const (
	cmdOsascript = "osascript"
	cmdOpen      = "open"
)

// trashItem moves a file/dir to the macOS Trash via osascript (safe delete).
var trashItem = func(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cleanedPath := filepath.Clean(path)
	script := fmt.Sprintf(`tell application "Finder" to delete POSIX file %q`, cleanedPath)

	// #nosec G204 -- The application intentionally constructs commands based on user input, and we've verified sanitization
	cmd := exec.CommandContext(ctx, cmdOsascript, "-e", script)
	return cmd.Run()
}

// openPath opens a file or directory with the default macOS app.
var openPath = func(ctx context.Context, path string) error {
	// #nosec G204 -- The application needs to open dynamic files
	return exec.CommandContext(ctx, cmdOpen, filepath.Clean(path)).Start()
}

// revealPath reveals an item in Finder.
var revealPath = func(ctx context.Context, path string) error {
	// #nosec G204 -- The application needs to open dynamic files
	return exec.CommandContext(ctx, cmdOpen, "-R", filepath.Clean(path)).Start()
}
