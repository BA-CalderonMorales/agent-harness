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
		Count:    -1, // no count: internal message tallies are not user signal
	})
	sections = append(sections, header)

	// Viewport for messages — an empty transcript gets the full
	// empty-state panel, not a bare hint line.
	vpContent := m.viewport.View()
	if strings.TrimSpace(vpContent) == "" {
		vpContent = chatEmptyState(m.persona)
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

	// No prompt glyph: the composer is the affordance, and the mode
	// line below states the mode. A prefix symbol is noise.
	editorWidth := columnWidth - 4
	if editorWidth < 20 {
		editorWidth = columnWidth
	}
	editorContent := m.textarea.View()

	editorPanel := InputEditorStyle.
		Width(editorWidth).
		Height(m.inputRows()).
		Render(editorContent)

	// The solid surface block covers the editor (and, transiently, inline
	// suggestions); it hugs the text so there is never a large slab of
	// background under where the user types. The agent's thinking state
	// lives in the message header, not here.
	var blockParts []string
	blockParts = append(blockParts, editorPanel)
	if m.showSuggestions && len(m.suggestions) > 0 {
		blockParts = append(blockParts, m.renderSuggestions())
	}

	// The composer's top border is the mode affordance: bright while
	// you can type (insert), dim while you read (navigate) — the
	// boundary is visible where the eyes already are, on the terminal's
	// own background.
	composerBorder := ColorBorder
	if m.modeLabel == "typing" || m.focused {
		composerBorder = ColorPrimary
	}
	blockPanel := InputContainerStyle.
		Width(columnWidth).
		BorderForeground(composerBorder).
		PaddingTop(ComposerTopPadding).
		PaddingBottom(ComposerBottomPadding).
		Render(lipgloss.JoinVertical(lipgloss.Left, blockParts...))

	// The mode line renders below the block, on the terminal background.
	composerPanel := lipgloss.JoinVertical(lipgloss.Left, blockPanel, m.renderModeLine())

	sections = append(sections, composerPanel)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderModeLine renders the mode · model · provider · reasoning-effort line
// shown under the input, mirroring modern composer status rows.
func (m ChatModel) renderModeLine() string {
	// The mode is always the first beat of the line: vim-style users read
	// navigate/typing instantly, and the persona never hides it.
	mode := m.modeLabel
	if mode == "" {
		mode = "navigate"
	}
	modeBit := ModePromptStyle.Render(mode)

	parts := []string{modeBit}
	if m.persona != "" {
		parts = append(parts, m.persona)
	}
	if m.model != "" {
		parts = append(parts, ShortenModelName(m.model))
	}
	if m.provider != "" {
		parts = append(parts, m.provider)
	}
	if m.agentMode != "" {
		parts = append(parts, ModePromptStyle.Render(m.agentMode))
	}
	effort := m.effort
	if effort == "" {
		effort = "medium"
	}
	parts = append(parts, "effort "+effort)

	line := InputMetaStyle.Render(strings.Join(parts, " · "))

	// While typing, the escape hatch rides on the same row,
	// right-aligned: "Esc" to navigate.
	if m.focused {
		hint := HelpDimStyle.Render(`"Esc" to navigate`)
		pad := m.width - lipgloss.Width(line) - lipgloss.Width(hint)
		if pad > 0 {
			line += strings.Repeat(" ", pad) + hint
		}
	}
	return line
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

// expandCaret is the visible affordance marking an expandable record:
// ▸ folds (click/Enter opens it), ▾ is open (click/Esc closes it).
func expandCaret(expanded bool) string {
	if expanded {
		return "▾"
	}
	return "▸"
}

// assistantReasoningRows reports the block-relative rows that carry the
// model's reasoning for an assistant message — the live preview line
// while thinking, or the expanded reasoning frame — so a click on those
// rows resolves back to the message. ok is false when the block exposes
// no reasoning rows. The row math must mirror renderAssistantMessage.
func (m ChatModel) assistantReasoningRows(msg ChatMessage) (start, lines int, ok bool) {
	const headerRows = 1
	if strings.TrimSpace(msg.Content) == "" && msg.Thinking {
		hintRows := 0
		if thinkingHint(m.elapsed) != "" {
			hintRows = 1
		}
		if m.expandedMessageID == msg.ID {
			if full := strings.TrimSpace(m.thinkingText); full != "" && !m.thinkingIsStatus {
				// Full reasoning plus the "esc to close" footer.
				return headerRows + hintRows, strings.Count(full, "\n") + 2, true
			}
			return 0, 0, false
		}
		if !m.thinkingIsStatus && reasoningPreview(m.thinkingText) != "" {
			return headerRows + hintRows, 1, true
		}
		return 0, 0, false
	}
	if m.expandedMessageID == msg.ID && strings.TrimSpace(msg.ReasoningText) != "" {
		// Frame header + reasoning + "esc to close" footer.
		return headerRows, strings.Count(msg.ReasoningText, "\n") + 3, true
	}
	return 0, 0, false
}

func (m ChatModel) renderAssistantMessage(msg ChatMessage) string {
	var b strings.Builder

	// Header
	header := AssistantStyle.Render("Agent")
	if !msg.Timestamp.IsZero() {
		header += TimestampStyle.Render(" " + msg.Timestamp.Format("15:04"))
	}
	// While the response is in progress the header carries a live status:
	// Agent 14:39 (6.2s) [8 chunks] (thinking ⠹) - the elapsed time ticks
	// from the model's clock, the chunk counter updates per chunk, and the
	// spinner animates on the same clock.
	elapsed := msg.ResponseTime
	if msg.Thinking {
		elapsed = m.elapsed
	}
	if elapsed > 0 {
		header += SuccessStyle.Render(fmt.Sprintf(" (%s)", formatElapsed(elapsed)))
	}
	if msg.Thinking {
		header += HelpDimStyle.Render(" ") + m.thinkingBadge(int(m.elapsed.Seconds())*4)
	}
	b.WriteString(header)
	b.WriteString("\n")

	// Content - render markdown for rich formatting (code blocks, bold,
	// italic, etc.). While thinking (before the first chunk) the bubble is
	// hidden so only the animated header shows. Once the first token has
	// been pending long enough to suggest a slow local model, an explanatory
	// progress line fills the gap. When reasoning deltas are streaming
	// (GLM/DeepSeek/Nemotron thinking), the tail of the reasoning text
	// previews under the badge — unless the record is expanded, which
	// shows the full reasoning like an expanded tool call.
	if strings.TrimSpace(msg.Content) == "" && msg.Thinking {
		if hint := thinkingHint(m.elapsed); hint != "" {
			b.WriteString(HelpDimStyle.Render(hint))
			b.WriteString("\n")
		}
		if m.expandedMessageID == msg.ID {
			if full := strings.TrimSpace(m.thinkingText); full != "" && !m.thinkingIsStatus {
				b.WriteString(HelpDimStyle.Render(full))
				b.WriteString("\n")
				b.WriteString(HelpDimStyle.Render("   └─ esc to close"))
				b.WriteString("\n")
			}
		} else if !m.thinkingIsStatus {
			if preview := reasoningPreview(m.thinkingText); preview != "" {
				// The caret advertises the click: the preview line opens
				// the full reasoning record.
				b.WriteString(HelpDimStyle.Render(expandCaret(false) + " " + preview))
				b.WriteString("\n")
			}
		}
		return b.String()
	}
	width := m.width - 4
	if width < 1 {
		width = 1
	}
	// Expanded reasoning record: the full model thinking, above the
	// answer — same interaction as an expanded tool call.
	if m.expandedMessageID == msg.ID && strings.TrimSpace(msg.ReasoningText) != "" {
		b.WriteString(ToolTimeStyle.Render("   ┌─ reasoning · esc to close"))
		b.WriteString("\n")
		b.WriteString(ToolTimeStyle.Render(msg.ReasoningText))
		b.WriteString("\n")
	}
	// The bubble adds a left border and padding (2 columns) on top of
	// Width(width): rendering at the full width makes lipgloss re-wrap
	// glamour's output and orphan words onto flush-left lines. Render
	// at the true inner column instead.
	renderedContent := renderMarkdown(msg.Content, width-2)
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

	// The expand caret advertises the click: ▸ folded (click opens),
	// ▾ open (click folds). formatToolContent reserves the two caret
	// columns so the right-aligned duration stays put.
	expanded := m.expandedMessageID != "" && m.expandedMessageID == msg.ID
	body := style.Render(expandCaret(expanded) + " " + msg.Content)

	// Expanded tool record: the full call beneath the summary line —
	// exactly what was called, no truncation. Esc (or clicking again)
	// folds it back.
	if expanded {
		body += "\n" + m.renderToolExpansion(msg)
	}
	return body
}

// renderToolExpansion renders the full call record for an expanded tool
// message: name, untruncated detail, and the raw input JSON.
func (m ChatModel) renderToolExpansion(msg ChatMessage) string {
	var b strings.Builder
	status := string(msg.ToolStatus)
	if msg.ToolElapsed > 0 {
		status += " · " + formatElapsed(msg.ToolElapsed)
	}
	b.WriteString(ToolTimeStyle.Render("   ┌─ " + msg.ToolDisplayName + " · " + status))
	if msg.ToolDetail != "" {
		b.WriteString("\n" + ToolTimeStyle.Render("   │  detail: ") + msg.ToolDetail)
	}
	if msg.ToolInputJSON != "" {
		b.WriteString("\n" + ToolTimeStyle.Render("   │  input:  ") + msg.ToolInputJSON)
	}
	b.WriteString("\n" + ToolTimeStyle.Render("   └─ esc to close"))
	return b.String()
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
