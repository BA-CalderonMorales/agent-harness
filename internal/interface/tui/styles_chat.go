package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Chat conversation
// ---------------------------------------------------------------------------
var (
	UserPromptStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true)

	AssistantStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	MessageStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	MarkdownBoldStyle = lipgloss.NewStyle().
				Foreground(ColorText).
				Bold(true)

	MarkdownItalicStyle = lipgloss.NewStyle().
				Foreground(ColorTextDim).
				Italic(true)

	CodeInlineStyle = lipgloss.NewStyle().
			Foreground(ColorAccent)

	CodeBlockStyle = lipgloss.NewStyle().
			Foreground(ColorText).
			Background(ColorHighlight)

	MessageBubbleUser = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderForeground(ColorSecondary).
				PaddingLeft(1)

	MessageBubbleAssistant = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderForeground(ColorPrimary).
				PaddingLeft(1)

	ToolCallStyle = lipgloss.NewStyle().
			Foreground(ColorAccent)

	// ToolCommandPreviewStyle - grey preview of actual command being executed
	// Used for human-readable command preview (like Kimi does)
	ToolCommandPreviewStyle = lipgloss.NewStyle().
				Foreground(ColorTextDim).
				Italic(true)

	ToolRunningStyle = lipgloss.NewStyle().
				Foreground(ColorInfo)

	ToolThinkingStyle = lipgloss.NewStyle().
				Foreground(ColorWarning)

	ToolDoneStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	ToolErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError)

	StreamingStyle = lipgloss.NewStyle().
			Foreground(ColorTextDim).
			Italic(true)

	SpinnerStyle = lipgloss.NewStyle().
			Foreground(ColorInfo)

	TimestampStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	SystemMessageStyle = lipgloss.NewStyle().
				Foreground(ColorWarning).
				Italic(true)

	SeparatorStyle = lipgloss.NewStyle().
			Foreground(ColorBorder)

	ScrollHintStyle = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true)
)

// ---------------------------------------------------------------------------
