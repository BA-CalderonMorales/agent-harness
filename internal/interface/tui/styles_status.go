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
	// The footer is the quiet tail of the bottom band: the composer's
	// full-width rule already marks where the content ends, so the
	// footer carries no rule of its own. The horizontal inset is applied
	// at render time so it matches the composer.
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
