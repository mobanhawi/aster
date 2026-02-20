package ui

import (
	"fmt"
	"os"
	"os/exec"

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
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case StateScanning:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}

	case StateConfirmDelete:
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

	case StateBrowsing:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			children := m.visibleChildren()
			if m.cursor < len(children)-1 {
				m.cursor++
			}

		case "right", "enter", "l":
			sel := m.selected()
			if sel != nil && sel.IsDir {
				m.stack = append(m.stack, sel)
				m.cursor = 0
			}

		case "left", "backspace", "h":
			if len(m.stack) > 0 {
				m.stack = m.stack[:len(m.stack)-1]
				m.clampCursor()
			}

		case "s":
			// Toggle sort mode
			if m.sort == SortBySize {
				m.sort = SortByName
				sortTree(m.root, SortByName)
			} else {
				m.sort = SortBySize
				sortTree(m.root, SortBySize)
			}
			m.cursor = 0

		case "o":
			// Open in Finder / default app
			sel := m.selected()
			if sel != nil {
				openPath(sel.Path)
			}

		case "r":
			// Reveal in Finder
			sel := m.selected()
			if sel != nil {
				revealPath(sel.Path)
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
			n := len(m.visibleChildren())
			if n > 0 {
				m.cursor = n - 1
			}
		}
	}

	return m, nil
}

// trashItem moves a file/dir to the macOS Trash via osascript (safe delete).
func trashItem(path string) error {
	script := fmt.Sprintf(`tell application "Finder" to delete POSIX file %q`, path)
	cmd := exec.Command("osascript", "-e", script)
	return cmd.Run()
}

// openPath opens a file or directory with the default macOS app.
func openPath(path string) {
	exec.Command("open", path).Start() //nolint:errcheck
}

// revealPath reveals an item in Finder.
func revealPath(path string) {
	exec.Command("open", "-R", path).Start() //nolint:errcheck
}

// removeDir removes a directory and all its contents (fallback for non-macOS or when Finder unavail).
func removeDir(path string) error {
	return os.RemoveAll(path)
}
