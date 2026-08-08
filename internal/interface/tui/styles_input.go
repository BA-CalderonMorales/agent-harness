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

	// InputContainerStyle - styled container for the input area.
	// No side padding: the centered composer column provides the breathing
	// room, keeping the input flush inside its border.
	InputContainerStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderTop(true).
				BorderForeground(ColorBorder).
				Padding(0, 0)

	// InputEditorStyle - the typing surface. Transparent (no background) so
	// the text sits directly on the terminal surface like a modern composer.
	InputEditorStyle = lipgloss.NewStyle().
				Padding(0, 0)

	InputMetaStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	PromptStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	InputHintStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)
)
