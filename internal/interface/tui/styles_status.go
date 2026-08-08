package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Status bar
// ---------------------------------------------------------------------------
var (
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(ColorTextDim).
			Padding(0, 1)

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
)

// ---------------------------------------------------------------------------
