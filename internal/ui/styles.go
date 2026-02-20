package ui

import "github.com/charmbracelet/lipgloss"

var (
	// Color palette
	colorBg     = lipgloss.AdaptiveColor{Dark: "#0f0f1a", Light: "#f5f5ff"}
	colorAccent = lipgloss.Color("#9b59b6")
	colorTeal   = lipgloss.Color("#1abc9c")
	colorDim    = lipgloss.Color("#444466")
	colorWhite  = lipgloss.Color("#e8e8f0")
	colorGray   = lipgloss.Color("#888899")
	colorRed    = lipgloss.Color("#e74c3c")
	colorOrange = lipgloss.Color("#e67e22")
	colorYellow = lipgloss.Color("#f1c40f")
	colorGreen  = lipgloss.Color("#2ecc71")

	// Bar colors by size percentile (index 0 = largest)
	barColors = []lipgloss.Color{
		colorRed,
		colorOrange,
		colorYellow,
		colorTeal,
		colorGreen,
		colorDim,
	}

	// Style: header bar
	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			Background(colorAccent).
			Padding(0, 2)

	// Style: breadcrumb path
	styleBreadcrumb = lipgloss.NewStyle().
			Foreground(colorTeal).
			Italic(true).
			Padding(0, 1)

	// Style: selected row highlight
	styleSelected = lipgloss.NewStyle().
			Background(lipgloss.Color("#2a1a4a")).
			Bold(true)

	// Style: normal row
	styleRow = lipgloss.NewStyle().
			Foreground(colorWhite)

	// Style: directory indicator
	styleDir = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	// Style: file indicator
	styleFile = lipgloss.NewStyle().
			Foreground(colorGray)

	// Style: size label (right-aligned)
	styleSize = lipgloss.NewStyle().
			Foreground(colorTeal).
			Width(9).
			Align(lipgloss.Right)

	// Style: size percentage
	stylePct = lipgloss.NewStyle().
			Foreground(colorGray).
			Width(5).
			Align(lipgloss.Right)

	// Style: footer bar
	styleFooter = lipgloss.NewStyle().
			Foreground(colorGray).
			Background(lipgloss.Color("#111122")).
			Padding(0, 1)

	// Style: key hint
	styleKey = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	// Style: scanning status
	styleScanning = lipgloss.NewStyle().
			Foreground(colorYellow).
			Bold(true)

	// Style: error
	styleError = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)

	// Style: confirm prompt
	styleConfirm = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true).
			Background(lipgloss.Color("#2a0000")).
			Padding(0, 2)

	// Style: info panel divider
	styleDivider = lipgloss.NewStyle().
			Foreground(colorDim)
)

// barColor picks a color based on the item's rank in the list.
func barColor(rank, total int) lipgloss.Color {
	if total <= 1 {
		return barColors[0]
	}
	idx := (rank * (len(barColors) - 1)) / (total - 1)
	if idx >= len(barColors) {
		idx = len(barColors) - 1
	}
	return barColors[idx]
}
