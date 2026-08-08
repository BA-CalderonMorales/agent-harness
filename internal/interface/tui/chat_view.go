package tui

import (
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/persona"
	"github.com/charmbracelet/lipgloss"
	"strings"
)

func (m ChatModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "  Initializing chat..."
	}

	m.syncTextareaHeight()
	inputHeight := m.inputAreaHeight()

	headerHeight := 2 // Header takes 2 lines
	separatorHeight := 1

	// Ensure minimum height for viewport
	vpHeight := m.height - inputHeight - headerHeight - separatorHeight
	if vpHeight < 5 {
		vpHeight = 5
	}

	// Ensure viewport has correct dimensions
	m.viewport.Width = m.width
	m.viewport.Height = vpHeight

	// Build the view
	var sections []string

	// Header (like Settings has)
	header := RenderHeader(HeaderConfig{
		Title:    "Chat",
		Subtitle: "Agent conversation",
		Count:    len(m.messages),
	})
	sections = append(sections, header)

	// Viewport for messages
	vpContent := m.viewport.View()
	if strings.TrimSpace(vpContent) == "" {
		hint := m.emptyStateHint()
		vpContent = HelpDimStyle.Render("  " + hint)
	}

	// Constrain viewport to calculated height
	vpRendered := lipgloss.NewStyle().
		Height(vpHeight).
		MaxHeight(vpHeight).
		Render(vpContent)
	sections = append(sections, vpRendered)

	// Composer: centered column with padding above and below the input text,
	// a mode line (mode · model · provider · reasoning effort) under it, and
	// optional inline suggestions between the editor and the mode line.
	columnWidth := m.width
	if columnWidth > ComposerColumnWidth {
		columnWidth = ComposerColumnWidth
	}

	prompt := PromptStyle.Render("◆ ")
	editorWidth := columnWidth - 4
	if editorWidth < 20 {
		editorWidth = columnWidth
	}
	editorContent := prompt + m.textarea.View()

	editorPanel := InputEditorStyle.
		Width(editorWidth).
		Height(m.inputRows()).
		Render(editorContent)

	var composerParts []string
	composerParts = append(composerParts, editorPanel)
	if m.thinking {
		composerParts = append(composerParts, m.renderStatusLine())
	}

	// Inline suggestions dropdown (below editor, above the gap)
	if m.showSuggestions && len(m.suggestions) > 0 {
		composerParts = append(composerParts, m.renderSuggestions())
	}

	for i := 0; i < ComposerGapRows; i++ {
		composerParts = append(composerParts, "")
	}
	composerParts = append(composerParts, m.renderModeLine())

	inputContent := lipgloss.JoinVertical(lipgloss.Left, composerParts...)
	composerPanel := InputContainerStyle.
		Width(columnWidth).
		PaddingTop(ComposerTopPadding).
		Render(inputContent)
	if m.width > columnWidth {
		composerPanel = lipgloss.PlaceHorizontal(m.width, lipgloss.Center, composerPanel)
	}

	sections = append(sections, composerPanel)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderModeLine renders the mode · model · provider · reasoning-effort line
// shown under the input, mirroring modern composer status rows.
func (m ChatModel) renderModeLine() string {
	mode := m.modeLabel
	if m.persona != "" {
		mode = m.persona
	} else if mode == "" {
		mode = "typing"
	}

	parts := []string{mode}
	if m.model != "" {
		parts = append(parts, ShortenModelName(m.model))
	}
	if m.provider != "" {
		parts = append(parts, m.provider)
	}
	effort := m.effort
	if effort == "" {
		effort = "medium"
	}
	parts = append(parts, "effort "+effort)

	return InputMetaStyle.Render(strings.Join(parts, " · "))
}

// syncSuggestionOffset keeps cursor inside visible window.
func (m ChatModel) renderMessage(msg ChatMessage) string {
	switch msg.Role {
	case "user":
		return m.renderUserMessage(msg)
	case "assistant":
		return m.renderAssistantMessage(msg)
	case "tool":
		return m.renderToolMessage(msg)
	case "system":
		return m.renderSystemMessage(msg)
	default:
		return msg.Content
	}
}

func (m ChatModel) renderUserMessage(msg ChatMessage) string {
	var b strings.Builder

	// Header
	header := UserPromptStyle.Render("You")
	if !msg.Timestamp.IsZero() {
		header += TimestampStyle.Render(" " + msg.Timestamp.Format("15:04"))
	}
	b.WriteString(header)
	b.WriteString("\n")

	// Content - render markdown for rich formatting
	width := m.width - 4
	if width < 1 {
		width = 1
	}
	renderedContent := renderMarkdown(msg.Content, width)
	content := MessageBubbleUser.Width(width).Render(renderedContent)
	b.WriteString(content)

	return b.String()
}

func (m ChatModel) renderAssistantMessage(msg ChatMessage) string {
	var b strings.Builder

	// Header
	header := AssistantStyle.Render("Agent")
	if !msg.Timestamp.IsZero() {
		header += TimestampStyle.Render(" " + msg.Timestamp.Format("15:04"))
	}
	// Show response time if available; while thinking (and after
	// completion) a bracketed thinking-time and chunk count rides along:
	// Agent 14:10 (1m7s) [1m7s | 45 chunks]
	if msg.ResponseTime > 0 {
		header += SuccessStyle.Render(fmt.Sprintf(" (%s)", formatElapsed(msg.ResponseTime)))
		if msg.StreamedChunks > 0 {
			header += HelpDimStyle.Render(fmt.Sprintf(" [%s | %d chunks]", formatElapsed(msg.ResponseTime), msg.StreamedChunks))
		}
	}
	b.WriteString(header)
	b.WriteString("\n")

	// Content - render markdown for rich formatting (code blocks, bold, italic, etc.)
	width := m.width - 4
	if width < 1 {
		width = 1
	}
	renderedContent := renderMarkdown(msg.Content, width)
	content := MessageBubbleAssistant.Width(width).Render(renderedContent)
	b.WriteString(content)

	return b.String()
}

func (m ChatModel) renderToolMessage(msg ChatMessage) string {
	// Choose style based on tool status
	var style lipgloss.Style
	switch msg.ToolStatus {
	case ToolStatusRunning:
		style = ToolRunningStyle
	case ToolStatusSuccess, ToolStatusComplete:
		style = ToolDoneStyle
	case ToolStatusError:
		style = ToolErrorStyle
	default:
		style = ToolCallStyle
	}

	// Content already has status indicator and command preview from formatToolContent
	return style.Render(msg.Content)
}

func (m ChatModel) renderSystemMessage(msg ChatMessage) string {
	return SystemMessageStyle.Render(msg.Content)
}

// emptyStateHint returns a contextual hint based on the current persona.
func (m ChatModel) emptyStateHint() string {
	p, err := persona.Parse(m.persona)
	if err != nil {
		return persona.Default().EmptyStateHint()
	}
	return p.EmptyStateHint()
}
