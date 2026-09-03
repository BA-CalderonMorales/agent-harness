package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Content area
// ---------------------------------------------------------------------------
var (
	ContentStyle     lipgloss.Style
	PanelStyle       lipgloss.Style
	DetailPanelStyle lipgloss.Style
)

func applyContentStyles() {
	ContentStyle = lipgloss.NewStyle().
		Padding(1, 2)

	PanelStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 2)

	DetailPanelStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1)
}

// ---------------------------------------------------------------------------
