package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Input area - Golazo-inspired styling
// ---------------------------------------------------------------------------
var (
	InputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	// InputContainerStyle - the composer panel. One solid surface from the
	// border through the editor, gap, and mode line, so the typing area
	// reads as a single consistent block instead of a background that only
	// appears behind the typed text.
	InputContainerStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderTop(true).
				BorderForeground(ColorBorder).
				Background(ColorSurface).
				Padding(0, 0)

	// InputEditorStyle - the typing surface, same solid background as the
	// container. The editor is rendered at a fixed max height below, so the
	// block never grows with the text.
	InputEditorStyle = lipgloss.NewStyle().
				Background(ColorSurface).
				Padding(0, 0)

	InputMetaStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	ModePromptStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	PromptStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary)

	InputHintStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)
)
