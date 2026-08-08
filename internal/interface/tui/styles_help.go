package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Help overlay
// ---------------------------------------------------------------------------
var (
	HelpTitleStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			Padding(0, 1)

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	HelpDimStyle = lipgloss.NewStyle().
			Foreground(ColorTextDim)

	HelpSectionSep = lipgloss.NewStyle().
			Foreground(ColorBorder)

	CategoryStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			MarginTop(1).
			MarginBottom(0)
)

// ---------------------------------------------------------------------------
