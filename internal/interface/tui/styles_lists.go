package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// List / table styles
// ---------------------------------------------------------------------------
var (
	ListTitleStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			Padding(0, 0, 1, 0)

	ListItemStyle = lipgloss.NewStyle().
			Foreground(ColorText).
			Padding(0, 2)

	ListSelectedStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary).
				Bold(true).
				Padding(0, 2).
				Background(ColorHighlight)

	ListDimStyle = lipgloss.NewStyle().
			Foreground(ColorTextDim).
			Padding(0, 2)

	ListHeaderStyle = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true).
			Underline(true)

	ListSeparatorStyle = lipgloss.NewStyle().
				Foreground(ColorBorder)
)

// ---------------------------------------------------------------------------
