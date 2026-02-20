package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	humanize "github.com/dustin/go-humanize"
)

// barFill and barDim are pre-built strings of the maximum bar width — we slice
// them to length instead of calling strings.Repeat on every row every frame.
const maxBarW = 30

var barFill = strings.Repeat("█", maxBarW) // sliced for the filled portion
var barDim = strings.Repeat("░", maxBarW)  // sliced for the empty portion

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

	// Show live scanned-bytes counter. We purposely avoid showing a percentage
	// here because we don't know the target directory's total size upfront —
	// any denominator (e.g. Statfs total) would be relative to the whole
	// filesystem volume rather than the scanned path, which is misleading.
	scanned := m.scannedBytes.Load()
	var progressHint string
	if scanned > 0 {
		progressHint = " (" + humanize.Bytes(uint64(scanned)) + " scanned)"
	}

	msg := styleScanning.Render("\n  " + m.sp.View() + " Scanning " + m.rootPath + "…" + progressHint + "\n")
	hint := styleFooter.Width(m.width).Render(" Press q to quit")
	return lipgloss.JoinVertical(lipgloss.Left, header, msg, hint)
}

// viewError renders an error screen.
func (m Model) viewError() string {
	header := styleHeader.Width(m.width).Render("  aster — Error")
	msg := styleError.Render("\n  ✗ " + m.scanErr.Error() + "\n")
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

	// ── Divider (cached by width) ─────────────────────────────────────────────
	lines = append(lines, m.divider())

	// ── File list ────────────────────────────────────────────────────────────
	children := m.visibleChildren()
	current := m.currentDir()
	totalSize := int64(0)
	if current != nil {
		totalSize = current.Size()
	}

	// Bar max width — capped globally, clamped for narrow terminals.
	barMaxW := m.width / 4
	if barMaxW > maxBarW {
		barMaxW = maxBarW
	}
	if barMaxW < 4 {
		barMaxW = 4
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
		row := m.renderRow(child, i, len(children), totalSize, barMaxW, i == m.cursor)
		lines = append(lines, row)
	}

	// Pad remaining rows
	for i := end - start; i < listHeight; i++ {
		lines = append(lines, "")
	}

	// ── Divider (cached) ──────────────────────────────────────────────────────
	lines = append(lines, m.divider())

	// ── Status bar ───────────────────────────────────────────────────────────
	sortLabel := "size"
	if m.sort == SortByName {
		sortLabel = "name"
	}
	n := len(children)
	// Use caches: humanSize avoids re-running humanize on every frame;
	// itoa avoids fmt.Sprintf for item count.
	statusLeft := " " + itoa(n) + " items  total: " + m.humanSize(totalSize) + "  sort: " + sortLabel
	statusRight := "scroll: " + scrollIndicator(m.cursor, n) + " "
	gap := m.width - utf8.RuneCountInString(statusLeft) - utf8.RuneCountInString(statusRight)
	if gap < 0 {
		gap = 0
	}
	statusLine := styleFooter.Render(statusLeft + strings.Repeat(" ", gap) + statusRight)
	lines = append(lines, statusLine)

	// ── Key hints (cached by width) ───────────────────────────────────────────
	lines = append(lines, m.keyHints())

	// ── Confirm-delete overlay ────────────────────────────────────────────────
	if m.state == StateConfirmDelete {
		name := filepath.Base(m.confirmPath)
		prompt := styleConfirm.Width(m.width).Render(
			"  ⚠  Move to Trash: " + truncate(name, m.width-50) + " ? [d/y/enter = yes  esc/n = no]",
		)
		lines = append(lines, prompt)
	}

	return strings.Join(lines, "\n")
}

// renderRow renders a single file/dir row.
// barMaxW is pre-computed by the caller to avoid repeating the clamping math.
func (m Model) renderRow(node *Node, rank, total int, parentSize int64, barMaxW int, selected bool) string {
	// Proportion of parent
	pct := 0.0
	if parentSize > 0 {
		pct = float64(node.Size()) / float64(parentSize)
	}
	barLen := int(pct * float64(barMaxW))
	if barLen == 0 && node.Size() > 0 {
		barLen = 1
	}
	if barLen > barMaxW {
		barLen = barMaxW
	}

	// Build bar by slicing pre-allocated strings to avoid strings.Repeat per row.
	filledPart := barFill[:barLen]
	dimPart := barDim[:barMaxW-barLen]
	bar := barStyle(rank, total).Render(filledPart) + styleBarDim.Render(dimPart)

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

	sz := node.Size()
	if sz < 0 {
		sz = 0
	}
	sizeStr := styleSize.Render(humanize.Bytes(uint64(sz)))
	pctStr := stylePct.Render(fmt.Sprintf("%4.0f%%", pct*100))

	row := bar + " " + name + sizeStr + pctStr

	if selected {
		return styleSelected.Width(m.width).Render(row)
	}
	return row
}

// breadcrumb returns a readable "~ › dir › subdir" path.
// Uses m.absRoot which was resolved once at scan time instead of calling
// filepath.Abs on every render frame.
func (m Model) breadcrumb() string {
	home := m.absRoot
	if home == "" {
		var err error
		home, err = filepath.Abs(m.rootPath)
		if err != nil {
			home = m.rootPath
		}
	}
	parts := make([]string, 0, len(m.stack)+1)
	parts = append(parts, " "+home)
	for _, n := range m.stack {
		parts = append(parts, n.Name)
	}
	return strings.Join(parts, " › ")
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
	return itoa(cursor+1) + "/" + itoa(total)
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
