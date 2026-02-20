package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	humanize "github.com/dustin/go-humanize"
)

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 {
		return "Initializing…"
	}

	switch m.state {
	case StateScanning:
		return m.viewScanning()
	case StateError:
		return m.viewError()
	case StateBrowsing, StateConfirmDelete:
		return m.viewBrowse()
	}
	return ""
}

// viewScanning renders the scanning progress screen.
func (m Model) viewScanning() string {
	header := styleHeader.Width(m.width).Render("  aster")
	msg := styleScanning.Render(fmt.Sprintf("\n  %s Scanning %s…\n", m.sp.View(), m.rootPath))
	hint := styleFooter.Width(m.width).Render(" Press q to quit")
	return lipgloss.JoinVertical(lipgloss.Left, header, msg, hint)
}

// viewError renders an error screen.
func (m Model) viewError() string {
	header := styleHeader.Width(m.width).Render("  aster — Error")
	msg := styleError.Render(fmt.Sprintf("\n  ✗ %v\n", m.scanErr))
	hint := styleFooter.Width(m.width).Render(" Press q to quit")
	return lipgloss.JoinVertical(lipgloss.Left, header, msg, hint)
}

// viewBrowse renders the main file browser screen.
func (m Model) viewBrowse() string {
	lines := make([]string, 0, m.height)

	// ── Header ──────────────────────────────────────────────────────────────
	lines = append(lines, styleHeader.Width(m.width).Render("  aster"))

	// ── Breadcrumb ───────────────────────────────────────────────────────────
	lines = append(lines, styleBreadcrumb.Width(m.width).Render(m.breadcrumb()))

	// ── Divider ──────────────────────────────────────────────────────────────
	lines = append(lines, styleDivider.Render(strings.Repeat("─", m.width)))

	// ── File list ────────────────────────────────────────────────────────────
	children := m.visibleChildren()
	current := m.currentDir()
	totalSize := int64(0)
	if current != nil {
		totalSize = current.Size()
	}

	// How many rows we can show (reserve header+breadcrumb+divider+footer+status = 6 rows)
	listHeight := m.height - 7
	if listHeight < 1 {
		listHeight = 1
	}

	// Viewport window: keep cursor visible
	start, end := scrollWindow(m.cursor, len(children), listHeight)

	for i := start; i < end; i++ {
		child := children[i]
		row := m.renderRow(child, i, len(children), totalSize, i == m.cursor)
		lines = append(lines, row)
	}

	// Pad remaining rows
	for i := end - start; i < listHeight; i++ {
		lines = append(lines, "")
	}

	// ── Divider ──────────────────────────────────────────────────────────────
	lines = append(lines, styleDivider.Render(strings.Repeat("─", m.width)))

	// ── Status bar ───────────────────────────────────────────────────────────
	sortLabel := "size"
	if m.sort == SortByName {
		sortLabel = "name"
	}
	statusLeft := fmt.Sprintf(" %d items  total: %s  sort: %s",
		len(children), humanize.Bytes(uint64(totalSize)), sortLabel)
	statusRight := "scroll: " + scrollIndicator(m.cursor, len(children)) + " "
	gap := m.width - utf8.RuneCountInString(statusLeft) - utf8.RuneCountInString(statusRight)
	if gap < 0 {
		gap = 0
	}
	statusLine := styleFooter.Render(statusLeft + strings.Repeat(" ", gap) + statusRight)
	lines = append(lines, statusLine)

	// ── Key hints ────────────────────────────────────────────────────────────
	hints := m.keyHints()
	lines = append(lines, styleFooter.Width(m.width).Render(hints))

	// ── Confirm-delete overlay ────────────────────────────────────────────────
	if m.state == StateConfirmDelete {
		name := filepath.Base(m.confirmPath)
		prompt := styleConfirm.Width(m.width).Render(
			fmt.Sprintf("  ⚠  Move to Trash: %s ? [d/y/enter = yes  esc/n = no]", truncate(name, m.width-50)),
		)
		lines = append(lines, prompt)
	}

	return strings.Join(lines, "\n")
}

// renderRow renders a single file/dir row.
func (m Model) renderRow(node *Node, rank, total int, parentSize int64, selected bool) string {
	// Bar width: reserve space for icon + name + size + pct
	barMaxW := m.width / 4
	if barMaxW > 30 {
		barMaxW = 30
	}
	if barMaxW < 4 {
		barMaxW = 4
	}

	// Proportion of parent
	pct := 0.0
	if parentSize > 0 {
		pct = float64(node.Size()) / float64(parentSize)
	}
	barLen := int(pct * float64(barMaxW))
	if barLen == 0 && node.Size() > 0 {
		barLen = 1
	}

	color := barColor(rank, total)
	bar := lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("█", barLen)) +
		lipgloss.NewStyle().Foreground(colorDim).Render(strings.Repeat("░", barMaxW-barLen))

	// Icon + name
	icon := styleFile.Render("  ")
	nameStyle := styleRow
	if node.IsDir {
		icon = styleDir.Render(" ")
		nameStyle = styleDir
	}
	if node.Err != nil {
		icon = styleError.Render("  ")
	}

	nameW := m.width - barMaxW - 18 // 18 = size(9) + pct(5) + gaps
	if nameW < 10 {
		nameW = 10
	}
	name := nameStyle.Width(nameW).Render(icon + truncate(node.Name, nameW-3))

	sizeStr := styleSize.Render(humanize.Bytes(uint64(node.Size())))
	pctStr := stylePct.Render(fmt.Sprintf("%4.0f%%", pct*100))

	row := bar + " " + name + sizeStr + pctStr

	if selected {
		return styleSelected.Width(m.width).Render(row)
	}
	return row
}

// breadcrumb returns a readable "~ > dir > subdir" path.
func (m Model) breadcrumb() string {
	home, _ := filepath.Abs(m.rootPath)
	parts := []string{" " + home}
	for _, n := range m.stack {
		parts = append(parts, n.Name)
	}
	return strings.Join(parts, " › ")
}

// keyHints returns the footer key hint string.
func (m Model) keyHints() string {
	k := func(key, desc string) string {
		return styleKey.Render(key) + " " + desc + "  "
	}
	return " " +
		k("↑↓/jk", "move") +
		k("→/enter", "enter") +
		k("←/bsp", "back") +
		k("o", "open") +
		k("r", "reveal") +
		k("d", "delete") +
		k("s", "sort") +
		k("q", "quit")
}

// scrollWindow returns [start, end) to keep cursor visible in listHeight rows.
func scrollWindow(cursor, total, height int) (int, int) {
	if total <= height {
		return 0, total
	}
	start := cursor - height/2
	if start < 0 {
		start = 0
	}
	end := start + height
	if end > total {
		end = total
		start = end - height
		if start < 0 {
			start = 0
		}
	}
	return start, end
}

// scrollIndicator shows a simple N/total indicator.
func scrollIndicator(cursor, total int) string {
	if total == 0 {
		return "0/0"
	}
	return fmt.Sprintf("%d/%d", cursor+1, total)
}

// truncate shortens a string with an ellipsis if it exceeds maxLen runes.
func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-1]) + "…"
}
