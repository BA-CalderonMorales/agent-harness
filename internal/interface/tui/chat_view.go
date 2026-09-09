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

	// Deferred rebuilds flush in Update (the persisted model), not
	// here: View has a value receiver, so mutations would not survive
	// the frame. Update() flushes before handling the message; the
	// viewport content View reads is therefore current. The one gap —
	// mutations made without an Update in between (none today; every
	// mutator path is reached from Update or from a delegate cmd that
	// lands as a message) — would paint one frame stale and correct
	// on the next Update.

	m.syncTextareaGeometry()
	inputHeight := m.inputAreaHeight()

	headerHeight := 2 // Header takes 2 lines
	separatorHeight := 1

	// Ensure minimum height for viewport
	// The live working row is part of the fixed chrome while a turn is in
	// flight. Reserve it before sizing the viewport; otherwise the row is
	// appended later and pushes the composer/mode line below the pane.
	statusHeight := m.workingStatusHeight()
	vpHeight := m.height - inputHeight - headerHeight - separatorHeight - statusHeight
	if vpHeight < 5 {
		vpHeight = 5
	}

	// Ensure viewport has correct dimensions
	m.viewport.Width = m.width
	m.viewport.Height = vpHeight
	// Composer top row in pane coordinates: the click mapper turns a
	// tap on the composer into a focus request (tap-to-type).
	m.lastComposerTop = viewportTopOffset + vpHeight

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

	// Agent working indicator: the live "what is the agent doing" line
	// above the composer (Codex parity). Renders in the chrome, not the
	// transcript; hidden entirely when idle so geometry is unchanged.
	tick := int(time.Since(m.startTime).Milliseconds() / 250)
	if status := m.renderWorkingStatus(tick, m.width); status != "" {
		sections = append(sections, status)
	}

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
	// own background. When the draft has rows scrolled out above the
	// window, the border says so: an overflow marker keeps the hidden
	// first line discoverable instead of silently gone.
	composerBorder := ColorBorder
	if m.modeLabel == "typing" || m.focused {
		composerBorder = ColorPrimary
	}
	if m.hiddenRowsAbove() > 0 {
		composerBorder = ColorPrimary
	}
	borderStyle := InputContainerStyle.
		Width(columnWidth).
		BorderForeground(composerBorder).
		PaddingTop(ComposerTopPadding).
		PaddingBottom(ComposerBottomPadding)
	if hidden := m.hiddenRowsAbove(); hidden > 0 {
		// The overflow marker rides the top rule: `… N lines above` tells
		// the driver exactly how much draft is out of view and that it is
		// reachable, not lost.
		marker := InputHintStyle.Render(
			"… " + fmt.Sprintf("%d line%s above — ↑ to scroll", hidden, plural(hidden)))
		block := lipgloss.JoinVertical(lipgloss.Left, marker, lipgloss.JoinVertical(lipgloss.Left, blockParts...))
		blockPanel := borderStyle.Render(block)
		composerPanel := lipgloss.JoinVertical(lipgloss.Left, blockPanel, m.renderModeLine())
		sections = append(sections, composerPanel)
		return lipgloss.JoinVertical(lipgloss.Left, sections...)
	}
	blockPanel := borderStyle.
		Render(lipgloss.JoinVertical(lipgloss.Left, blockParts...))

	// The mode line renders below the block, on the terminal background.
	composerPanel := lipgloss.JoinVertical(lipgloss.Left, blockPanel, m.renderModeLine())

	sections = append(sections, composerPanel)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// plural returns "s" for counts other than one.
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
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
		budget -= 3 + segments[i].width // separator + segment
	}
	// Keep the mode and effort signal useful on a narrow pane. Drop
	// optional context first: provider, persona, then model. Effort is
	// the last metadata field to disappear because it describes the
	// active behavior rather than the selected implementation.
	for _, i := range []int{2, 0, 1, 3} {
		if budget >= 0 {
			break
		}
		if i >= len(segments) {
			continue
		}
		if keep[i] {
			keep[i] = false
			budget += segments[i].width + 3
		}
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

	// Symmetric hints on the same row, right-aligned. While typing,
	// "Esc" is the exit hatch; while navigating, "i" (or a tap on
	// mobile panes, which have no `i` affordance) re-enters typing.
	if m.focused {
		hint := HelpDimStyle.Render(`"Esc" to navigate`)
		pad := m.width - lipgloss.Width(line) - lipgloss.Width(hint)
		if pad > 0 {
			line += strings.Repeat(" ", pad) + hint
		}
	} else {
		word := `"i" to type`
		if isMobilePane(m.width) {
			// Mobile panes enter the composer by tap-to-type
			// (chat_update.go mouse handling), not via `i`.
			word = `tap to type`
		}
		hint := HelpDimStyle.Render(word)
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

// assistantReasoningRows is superseded by the click refs constructed
// while rendering (assistantInnerContent): row estimates from the raw
// text drift the moment a segment wraps inside the bubble.

func (m ChatModel) renderAssistantMessage(msg ChatMessage) string {
	rendered, _ := m.renderAssistantTracked(msg, m.width)
	return rendered
}

// renderAssistantTracked renders the assistant block — header row, then
// the answer bubble — and reports the clickable ranges inside it,
// relative to the block's first row. The bubble is left-border only, so
// inner rows map onto bubble rows one-to-one; only the header offsets.
//
// A live toolRow resolver is wired here (goal 0.3.29 live-visibility):
// the streaming assistant carries tool parts as calls come in, and
// they must render inside the live bubble. The resolver looks the call
// up by ID across the transcript so the row reflects the message's
// current state (running → settled) without the turn-block machinery.
func (m ChatModel) renderAssistantTracked(msg ChatMessage, width int) (string, []clickRef) {
	toolRow := func(id string) (string, bool) {
		for k := range m.messages {
			tm := &m.messages[k]
			if tm.ID == id && tm.IsTool {
				return m.renderToolMessageAt(*tm, width-8), true
			}
		}
		return "", false
	}
	inner, refs := m.assistantInnerContent(msg, msg.Parts, toolRow)
	bubbles := MessageBubbleAssistant.Width(width - 4).Render(inner)
	return m.renderAssistantHeader(msg) + "\n" + bubbles, offsetClickRefs(refs, 1)
}

// renderAssistantHeader is the "Agent 22:24" line — split from the
// content so a turn block can nest its tool calls between the two.
// Live turn state (elapsed clock, thinking badge) moved to the working
// indicator above the composer: the header is the record ("this reply
// landed at 22:24"), the status line is the live signal. On finalize,
// the settled ResponseTime renders again.
func (m ChatModel) renderAssistantHeader(msg ChatMessage) string {
	var b strings.Builder

	header := AssistantStyle.Render("Agent")
	if !msg.Timestamp.IsZero() {
		header += TimestampStyle.Render(" " + chatStamp(msg.Timestamp))
	}
	// Live turns render a bare header; the elapsed clock and activity
	// live in the working indicator above the composer (chat_working.go).
	// Settled turns show how long the response took.
	if !msg.Thinking && msg.ResponseTime > 0 {
		header += SuccessStyle.Render(fmt.Sprintf(" (%s)", formatElapsed(msg.ResponseTime)))
	}
	b.WriteString(header)
	return b.String()
}

// assistantInnerContent builds the bubble's inner lines: thinking
// hints, the reasoning frame, then the response — split where tool
// calls interrupted it, with each call's row injected at its position.
// toolRow resolves a tool part to its rendered row; nil skips tools.
// Every dynamic segment is pre-wrapped to the bubble's inner width, so
// the rendered row count equals the count the click refs report: a
// segment that wrapped inside the style would silently shift every
// row below it.
func (m ChatModel) assistantInnerContent(msg ChatMessage, parts []TurnPart, toolRow func(id string) (string, bool)) (string, []clickRef) {
	var b strings.Builder
	var refs []clickRef
	rows := 0 // running row count of what has been written

	// Content - render markdown for rich formatting (code blocks, bold,
	// italic, etc.). While thinking (before the first chunk) the bubble is
	// hidden so only the animated header shows — UNLESS the turn already
	// carries tool calls: those must render as they come in (goal 0.3.29
	// live-visibility finding — a tool inside the placeholder window used
	// to hide behind this early return, leaving the user blind while the
	// command ran). When reasoning deltas are streaming (GLM/DeepSeek/
	// Nemotron thinking), the tail of the reasoning text previews under
	// the badge — unless the record is expanded, which shows the full
	// reasoning like an expanded tool call.
	hasToolParts := false
	for _, p := range parts {
		if p.ToolID != "" {
			hasToolParts = true
			break
		}
	}
	if strings.TrimSpace(msg.Content) == "" && msg.Thinking && !hasToolParts {
		if m.expandedMessageID == msg.ID {
			if full := strings.TrimSpace(m.thinkingText); full != "" && !m.thinkingIsStatus {
				wrapped := fitBlock(m.width-4, full)
				b.WriteString(HelpDimStyle.Render(wrapped))
				b.WriteString("\n")
				refs = append(refs, clickRef{start: rows, lines: strings.Count(wrapped, "\n") + 1, msgID: msg.ID})
				b.WriteString(HelpDimStyle.Render("   └─ esc to close"))
				b.WriteString("\n")
			}
		} else if !m.thinkingIsStatus {
			if preview := reasoningPreview(m.thinkingText); preview != "" {
				// The caret advertises the click: the preview line opens
				// the full reasoning record.
				b.WriteString(HelpDimStyle.Render(expandCaret(false) + " " + preview))
				b.WriteString("\n")
				refs = append(refs, clickRef{start: rows, lines: 1, msgID: msg.ID})
			}
		}
		return b.String(), refs
	}
	width := m.width - 4
	if width < 1 {
		width = 1
	}
	// Expanded reasoning record: the full model thinking, above the
	// answer — same interaction as an expanded tool call. The frame
	// closes with the └─ footer like a tool record and the live frame.
	if m.expandedMessageID == msg.ID && strings.TrimSpace(msg.ReasoningText) != "" {
		b.WriteString(ToolTimeStyle.Render("   ┌─ reasoning · esc to close"))
		b.WriteString("\n")
		wrapped := fitBlock(width-2, msg.ReasoningText)
		b.WriteString(ToolTimeStyle.Render(wrapped))
		b.WriteString("\n")
		refs = append(refs, clickRef{start: rows, lines: strings.Count(wrapped, "\n") + 2, msgID: msg.ID})
		rows += strings.Count(wrapped, "\n") + 2
		b.WriteString(ToolTimeStyle.Render("   └─ esc to close"))
		b.WriteString("\n")
		rows++
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
					refs = append(refs, clickRef{start: rows, lines: strings.Count(row, "\n") + 1, msgID: part.ToolID})
					rows += strings.Count(row, "\n") + 1
				}
				continue
			}
			if strings.TrimSpace(part.Text) == "" {
				continue
			}
			rendered := renderMarkdown(part.Text, width-2)
			b.WriteString(rendered)
			b.WriteString("\n")
			rows += strings.Count(rendered, "\n") + 1
		}
		return strings.TrimRight(b.String(), "\n"), refs
	}
	renderedContent := renderMarkdown(msg.Content, width-2)
	b.WriteString(renderedContent)

	return strings.TrimRight(b.String(), "\n"), refs
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

	// A todo-list call renders as a visible checklist (Task 4.4): the
	// summary row stays (time, glyph, name, duration), and each todo
	// becomes an indented checkbox row beneath it.
	if rows := m.todoChecklistRows(msg); rows != nil {
		body := style.Render(expandCaret(expanded) + " " + m.formatToolContentAt(width, msg.ToolDisplayName, msg.ToolDetail, shortToolTag(msg.ID), msg.ToolStatus, msg.ToolStartedAt, msg.ToolElapsed))
		for _, r := range rows {
			body += "\n " + r
		}
		if expanded {
			body += "\n" + fitBlock(width, m.renderToolExpansion(msg))
		}
		return body
	}

	row := m.formatToolContentAt(width, msg.ToolDisplayName, msg.ToolDetail, shortToolTag(msg.ID), msg.ToolStatus, msg.ToolStartedAt, msg.ToolElapsed)
	body := style.Render(expandCaret(expanded) + " " + row)

	// Expanded tool record: the full call beneath the summary line —
	// exactly what was called, no truncation. Esc (or clicking again)
	// folds it back. The record is wrapped to the width budget: a row
	// wider than the pane would wrap on the terminal and shift every
	// row below it.
	if expanded {
		body += "\n" + fitBlock(width, m.renderToolExpansion(msg))
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
