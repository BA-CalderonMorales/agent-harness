package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"strings"
	"time"
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

	// Viewport for messages — a conversation with no turns yet gets
	// the full empty-state panel, centered in the pane. System notices
	// (new-chat notes, session loads) do not count as conversation:
	// they render as their own lines above the panel. The viewport is
	// bypassed here — it pads to its own height, and its phantom blank
	// rows would push the centered panel past MaxHeight's clip.
	vpContent := m.viewport.View()
	if !m.hasConversation() {
		var notices []string
		for _, msg := range m.messages {
			if msg.Role == "system" && strings.TrimSpace(msg.Content) != "" {
				notices = append(notices, SystemMessageStyle.Render(msg.Content))
			}
		}
		noticeRows := len(notices)
		panel := chatEmptyState(m.persona, m.width, vpHeight-noticeRows-1)
		if len(notices) > 0 {
			panel = strings.Join(notices, "\n") + "\n\n" + panel
		}
		vpContent = panel
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

	// Segment priority when the pane narrows: the mode leads, the
	// model identifies what is answering; persona, effort, and provider
	// yield in that order. A clipped mid-word line helps nobody.
	type segment struct {
		width int
		text  string
		style bool
	}
	segments := []segment{}
	if m.persona != "" {
		segments = append(segments, segment{lipgloss.Width(m.persona), m.persona, false})
	}
	if m.model != "" {
		name := ShortenModelName(m.model)
		segments = append(segments, segment{lipgloss.Width(name), name, false})
	}
	if m.provider != "" {
		segments = append(segments, segment{lipgloss.Width(m.provider), m.provider, false})
	}
	if m.agentMode != "" {
		segments = append(segments, segment{lipgloss.Width(m.agentMode), m.agentMode, true})
	}
	effort := m.effort
	if effort == "" {
		effort = "medium"
	}
	segments = append(segments, segment{lipgloss.Width("effort " + effort), "effort " + effort, false})

	budget := m.width - lipgloss.Width(mode)
	keep := make([]bool, len(segments))
	for i := range segments {
		keep[i] = true
		budget -= 3 // separator
		if i > 0 {
			budget -= segments[i].width
		}
	}
	// Drop from the least important (effort) up while over budget.
	for i := len(segments) - 1; i >= 0 && budget < 0; i-- {
		keep[i] = false
		budget += segments[i].width + 3
	}
	parts := []string{modeBit}
	for i := range segments {
		if !keep[i] {
			continue
		}
		text := segments[i].text
		switch {
		case segments[i].style && text == "auto":
			// Yolo wears a warning: tools run without asking, and the
			// chip should read as a state, not a decoration.
			text = WarningStyle.Render(text)
		case segments[i].style:
			text = ModePromptStyle.Render(text)
		}
		parts = append(parts, text)
	}
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
	// A wrapped row is unbudgeted height: truncation keeps the frame
	// inside the terminal on narrow screens.
	return lipgloss.NewStyle().MaxWidth(m.width).Render(line)
}

// syncSuggestionOffset keeps cursor inside visible window.
// renderMessageAt renders one message for a width budget — nested
// tool rows live inside the response bubble, narrower than the pane.
func (m ChatModel) renderMessageAt(msg ChatMessage, width int) string {
	switch msg.Role {
	case "user":
		return m.renderUserMessage(msg)
	case "assistant":
		return m.renderAssistantMessage(msg)
	case "tool":
		return m.renderToolMessageAt(msg, width)
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
		header += TimestampStyle.Render(" " + chatStamp(msg.Timestamp))
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
	return m.renderAssistantHeader(msg) + "\n" + m.renderAssistantContent(msg)
}

// renderAssistantHeader is the "Agent 22:24 (22.3s)" line — split from
// the content so a turn block can nest its tool calls between the two.
func (m ChatModel) renderAssistantHeader(msg ChatMessage) string {
	var b strings.Builder

	// Header
	header := AssistantStyle.Render("Agent")
	if !msg.Timestamp.IsZero() {
		header += TimestampStyle.Render(" " + chatStamp(msg.Timestamp))
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
	return b.String()
}

// renderAssistantContent is the answer bubble: markdown, thinking
// hints, the reasoning preview, and the expanded reasoning record.
func (m ChatModel) renderAssistantContent(msg ChatMessage) string {
	inner := m.assistantInnerContent(msg, msg.Parts, nil)
	return MessageBubbleAssistant.Width(m.width - 4).Render(inner)
}

// assistantInnerContent builds the bubble's inner lines: thinking
// hints, the reasoning frame, then the response — split where tool
// calls interrupted it, with each call's row injected at its position.
// toolRow resolves a tool part to its rendered row; nil skips tools.
func (m ChatModel) assistantInnerContent(msg ChatMessage, parts []TurnPart, toolRow func(id string) (string, bool)) string {
	var b strings.Builder

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
	// The response renders part by part: prose as markdown, and each
	// tool call's row where it actually happened. Legacy messages
	// (no Parts) render Content whole.
	if len(parts) > 0 && toolRow != nil {
		for _, part := range parts {
			if part.ToolID != "" {
				if row, ok := toolRow(part.ToolID); ok {
					b.WriteString(row)
					b.WriteString("\n")
				}
				continue
			}
			if strings.TrimSpace(part.Text) == "" {
				continue
			}
			b.WriteString(renderMarkdown(part.Text, width-2))
			b.WriteString("\n")
		}
		return strings.TrimRight(b.String(), "\n")
	}
	renderedContent := renderMarkdown(msg.Content, width-2)
	b.WriteString(renderedContent)

	return strings.TrimRight(b.String(), "\n")
}

// renderToolMessageAt renders the tool row for a width budget.
func (m ChatModel) renderToolMessageAt(msg ChatMessage, width int) string {
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
	row := m.formatToolContentAt(width+2, msg.ToolDisplayName, msg.ToolDetail, msg.ToolStatus, msg.ToolStartedAt, msg.ToolElapsed)
	body := style.Render(expandCaret(expanded) + " " + row)

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
		for _, line := range prettyInputJSON(msg.ToolInputJSON) {
			b.WriteString("\n" + ToolTimeStyle.Render("   │  "+line))
		}
	}
	b.WriteString("\n" + ToolTimeStyle.Render("   └─ esc to close"))
	return b.String()
}

// chatStamp renders a message time: today shows the clock, anything
// older carries its date — overnight sessions keep their history
// readable.
func chatStamp(t time.Time) string {
	if t.Local().Format(dayKeyFormat) == time.Now().Local().Format(dayKeyFormat) {
		return t.Format("15:04")
	}
	return t.Format("Jan 02 15:04")
}

// prettyInputJSON formats the raw tool input for the expansion frame:
// indented JSON when it parses, the raw string when it does not.
func prettyInputJSON(raw string) []string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(raw), "", "  "); err == nil {
		return strings.Split(buf.String(), "\n")
	}
	return strings.Split(raw, "\n")
}

func (m ChatModel) renderSystemMessage(msg ChatMessage) string {
	return SystemMessageStyle.Render(msg.Content)
}

// hasConversation reports whether the transcript holds anything the
// agent or the user said — the empty state's trigger.
func (m ChatModel) hasConversation() bool {
	for _, msg := range m.messages {
		if msg.Role == "user" || msg.Role == "assistant" || msg.IsTool {
			return true
		}
	}
	return false
}
