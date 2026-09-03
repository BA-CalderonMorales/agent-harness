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

	// InputContainerStyle - the composer. No background paint: a strong
	// top rule and the mode line below bound the typing area, and the
	// terminal's own background carries it — the leanest surface there
	// is. The border brightens with focus (see chat_view).
	InputContainerStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderTop(true).
				BorderForeground(ColorBorder).
				Padding(0, 0)

	// InputEditorStyle - the typing surface, transparent like the rest.
	InputEditorStyle = lipgloss.NewStyle()

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
