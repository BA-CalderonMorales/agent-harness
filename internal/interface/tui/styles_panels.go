package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Design System - Panel styles
// ---------------------------------------------------------------------------
var (
	PanelPrimary    lipgloss.Style
	PanelSecondary  lipgloss.Style
	PanelHighlight  lipgloss.Style
	HeaderPrimary   lipgloss.Style
	HeaderSecondary lipgloss.Style
	HeaderTertiary  lipgloss.Style
	// FrameStyle bounds the whole interface: a one-cell rule on every
	// side in the composer rule's color, so the transcript bubbles read
	// as inset from the terminal edge. No padding of its own — the
	// views carry the inset via resize's frame reserve.
	FrameStyle lipgloss.Style
)

func applyPanelStyles() {
	FrameStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(ColorBorder)
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

	// Header styles
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
}
