package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Status bar
// ---------------------------------------------------------------------------
var (
	StatusBarStyle   lipgloss.Style
	StatusOnline     lipgloss.Style
	StatusOffline    lipgloss.Style
	StatusLabel      lipgloss.Style
	StatusHintStyle  lipgloss.Style
	StatusConnecting lipgloss.Style
)

func applyStatusStyles() {
	// Flush left/right (no interior padding) so the footer's text edges
	// align exactly with the composer block above it.
	StatusBarStyle = lipgloss.NewStyle().
		Foreground(ColorTextDim).
		Padding(0, 0)

	StatusOnline = lipgloss.NewStyle().
		Foreground(ColorSuccess).
		Bold(true)

	StatusOffline = lipgloss.NewStyle().
		Foreground(ColorError).
		Bold(true)

	StatusLabel = lipgloss.NewStyle().
		Foreground(ColorTextDim)

	StatusHintStyle = lipgloss.NewStyle().
		Foreground(ColorMuted)

	StatusConnecting = lipgloss.NewStyle().
		Foreground(ColorWarning).
		Bold(true)
}

// ---------------------------------------------------------------------------
