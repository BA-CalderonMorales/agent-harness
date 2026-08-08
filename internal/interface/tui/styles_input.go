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

	// InputContainerStyle - styled container for the input area
	// CRITICAL FIX: Consistent background, no strange color changes
	InputContainerStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderTop(true).
				BorderForeground(ColorBorder).
				Padding(0, 1)

	InputEditorStyle = lipgloss.NewStyle().
				Background(ColorSurface).
				Padding(0, 1)

	InputMetaStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	PromptStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	InputHintStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)
)
