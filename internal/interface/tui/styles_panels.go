package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Design System - Panel styles
// ---------------------------------------------------------------------------
var (
	PanelPrimary = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2)

	PanelSecondary = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(ColorMuted).
			Padding(0, 1)

	PanelHighlight = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Background(ColorHighlight).
			Padding(1, 2)
)

// Header styles
var (
	HeaderPrimary = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			MarginBottom(1)

	HeaderSecondary = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true)

	HeaderTertiary = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true)
)
