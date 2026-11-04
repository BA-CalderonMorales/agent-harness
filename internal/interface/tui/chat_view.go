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

	const headerHeight = 2 // header + its blank row
	separatorHeight := 1
	statusHeight := m.workingStatusHeight()
	// inputAreaHeight already reserves the composer and, while a turn is
	// live, the status band inside it.
	inputHeight := m.inputAreaHeight()

	// The composer is non-negotiable: losing it strands the driver with
	// no way to type. So the chrome is measured, and on a pane too short
	// to carry all of it the decoration yields first — the header, then
	// the live status band — before the transcript is squeezed below a
	// single row. The old code floored the transcript at five rows
	// regardless of what was left, which pushed the view over its budget;
	// the app frame's clip then silently ate the working indicator and
	// the entire composer. A 100x16 pane lost both.
	showStatus := statusHeight > 0
	showHeader := true
	chromeRows := func() int {
		rows := inputHeight + separatorHeight
		if showHeader {
			rows += headerHeight
		}
		if !showStatus {
			// inputAreaHeight counts the band; it is not rendered.
			rows -= statusHeight
		}
		return rows
	}
	for m.height-chromeRows() < 1 && (showHeader || showStatus) {
		if showHeader {
			showHeader = false
			continue
		}
		showStatus = false
	}

	vpHeight := m.height - chromeRows()
	if vpHeight < 1 {
		vpHeight = 1
	}

	// The live working band changes the viewport height between frames.
	// The stored offset was computed for the previous height, so keeping
	// it can hide the newest rows under the chrome — the streamed answer
	// would be clipped exactly while it is arriving. Follow the bottom
	// when the reader was there; otherwise clamp an offset that no longer
	// fits.
	atBottom := m.viewport.AtBottom()
	m.viewport.Width = m.width
	m.viewport.Height = vpHeight
	if atBottom {
		m.viewport.GotoBottom()
	} else {
		m.viewport.SetYOffset(m.viewport.YOffset)
	}
	// Composer top row in pane coordinates: the click mapper turns a
	// tap on the composer into a focus request (tap-to-type). The live
	// working block sits between the viewport and the composer.
	statusRows := 0
	if showStatus {
		statusRows = statusHeight
	}
	m.lastComposerTop = viewportTopOffset + vpHeight + statusRows

	// Build the view
	var sections []string

	// Header (like Settings has) — yielded on a pane too short to carry
	// both it and a row of transcript.
	if showHeader {
		header := RenderHeader(HeaderConfig{
			Title:    "Chat",
			Subtitle: "Agent conversation",
			Count:    -1, // no count: internal message tallies are not user signal
		})
		sections = append(sections, header)
	}

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
	if showStatus {
		if status := m.renderWorkingStatus(tick, m.composerWidth()); status != "" {
			// One row, directly above the composer rule: the beat track
			// is a reading, not a conversation turn. The live spinner and
			// clock ride on the Agent header now, so the band no longer
			// needs padding — and the reclaimed rows go to the transcript.
			status = lipgloss.NewStyle().Width(m.width).Padding(0, chromeInset(m.width)).Render(status)
			sections = append(sections, status)
		}
	}

	// Composer: a full-width rule closes the transcript, then the input
	// and mode line sit in an inset column under it — the desktop-app
	// shape of a bottom toolbar rather than a floating box.
	columnWidth := m.width
	composerWidth := m.composerWidth()
	inset := chromeInset(m.width)

	// No prompt glyph: the composer is the affordance, and the mode
	// line below states the mode. A prefix symbol is noise.
	editorWidth := composerWidth - 4
	if editorWidth < 20 {
		editorWidth = composerWidth
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

	// The composer's top border is the footer band's rule: full width,
	// in the same accent as the tab bar's rule, so the top and bottom
	// chrome read as one frame around the content. The typing affordance
	// lives in the mode line below; the rule never changes color and
	// leaves no floating gap at either edge.
	borderStyle := InputContainerStyle.
		Width(columnWidth).
		BorderForeground(ColorPrimary).
		Padding(0, inset).
		PaddingTop(ComposerTopPadding).
		PaddingBottom(ComposerBottomPadding)

	// The solid block hugs the text; when the draft overflows, a tail
	// marker rides under it: `… N lines below` tells the driver exactly
	// how much draft is out of view while the first line stays visible.
	var block []string
	block = append(block, blockParts...)
	if hidden := m.hiddenRowsBelow(); hidden > 0 {
		marker := InputHintStyle.Render(
			"… " + fmt.Sprintf("%d line%s below — ↓ to scroll", hidden, plural(hidden)))
		block = append(block, marker)
	}
	blockPanel := borderStyle.
		Render(lipgloss.JoinVertical(lipgloss.Left, block...))

	// The mode line renders below the block, on the terminal background.
	// It takes the composer's inset without taking the rule's: the text
	// keeps its breathing room while the rule spans the footer's edges.
	modeLine := lipgloss.NewStyle().
		Width(columnWidth).
		Padding(0, inset).
		Render(m.renderModeLine())
	composerPanel := lipgloss.JoinVertical(lipgloss.Left, blockPanel, modeLine)
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

	// Content - render markdown for rich formatting. Trim the renderer's
	// outer newlines: they painted a blank gutter row above and below
	// every bubble, so a one-line message carried two empty border rows.
	width := m.bubbleWidth()
	renderedContent := strings.Trim(renderMarkdown(msg.Content, width), "\n")
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
				return m.renderToolMessageAt(*tm, m.toolRowWidth(bubbleChromeCols)), true
			}
		}
		return "", false
	}
	inner, refs := m.assistantInnerContent(msg, msg.Parts, toolRow)
	header := m.renderAssistantHeader(msg)
	if strings.TrimSpace(inner) == "" {
		// A live placeholder before the first token is only its header:
		// an empty bubble would paint a lone gutter row and push the
		// question it answers out of the viewport.
		return header, nil
	}
	bubbles := MessageBubbleAssistant.Width(m.bubbleWidth()).Render(inner)
	return header + "\n" + bubbles, offsetClickRefs(refs, 1)
}

// renderAssistantHeader is the "Agent 22:24" line — split from the
// content so a turn block can nest its tool calls between the two.
// A live turn carries its spinner and phase here, at the bubble that is
// being cooked: the eye is already where the response will land, and
// the chrome band is left to the loop track. On finalize, the settled
// ResponseTime replaces the live phase.
func (m ChatModel) renderAssistantHeader(msg ChatMessage) string {
	var b strings.Builder

	header := AssistantStyle.Render("Agent")
	if !msg.Timestamp.IsZero() {
		header += TimestampStyle.Render(" " + chatStamp(msg.Timestamp))
	}
	if msg.Thinking {
		tick := int(m.elapsed.Milliseconds() / 250)
		header += HelpDimStyle.Render(" · ") + m.liveHeaderStatus(tick)
	} else if msg.ResponseTime > 0 {
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
	width := m.bubbleWidth()
	// Expanded reasoning record: the full model thinking, above the
	// answer — same interaction as an expanded tool call. The frame
	// closes with the └─ footer like a tool record and the live frame.
	if m.expandedMessageID == msg.ID && strings.TrimSpace(msg.ReasoningText) != "" {
		// The same frame shape as an expanded tool record: the box hangs
		// two columns in, and every body line carries the │ gutter. The
		// body used to render flush-left against a three-column header
		// and footer, which read as a floating label above a stray
		// paragraph rather than one record.
		b.WriteString(ToolTimeStyle.Render("  ┌─ reasoning · esc to close"))
		b.WriteString("\n")
		wrapped := fitBlock(width-5, msg.ReasoningText)
		for _, line := range strings.Split(wrapped, "\n") {
			b.WriteString(ToolTimeStyle.Render("  │  " + line))
			b.WriteString("\n")
		}
		refs = append(refs, clickRef{start: rows, lines: strings.Count(wrapped, "\n") + 2, msgID: msg.ID})
		rows += strings.Count(wrapped, "\n") + 2
		b.WriteString(ToolTimeStyle.Render("  └─ esc to close"))
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
			rendered := strings.Trim(renderMarkdown(part.Text, width-2), "\n")
			b.WriteString(rendered)
			b.WriteString("\n")
			rows += strings.Count(rendered, "\n") + 1
		}
		return strings.TrimRight(b.String(), "\n"), refs
	}
	// The caret marks the live writing edge: the answer is arriving,
	// not waiting. It is part of the markdown source so it sits at the
	// actual text edge; appending it to the rendered, width-padded line
	// would park it at the far right of the bubble.
	content := msg.Content
	if msg.Thinking && m.writing {
		content += "▍"
	}
	renderedContent := strings.Trim(renderMarkdown(content, width-2), "\n")
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
	// ▾ open (click folds). It renders in the muted time style rather
	// than the status style — an affordance reads as chrome, and when it
	// took the status colour it read as another status glyph crammed
	// against the timestamp. The row's own budget leaves the two caret
	// columns alone, so the right-aligned duration stays put.
	expanded := m.expandedMessageID != "" && m.expandedMessageID == msg.ID
	caret := ToolTimeStyle.Render(expandCaret(expanded)) + " "

	// A todo-list call renders as a visible checklist (Task 4.4): the
	// summary row stays (time, glyph, name, duration), and each todo
	// becomes an indented checkbox row beneath it.
	if rows := m.todoChecklistRows(msg); rows != nil {
		body := caret + style.Render(m.formatToolContentAt(width, msg.ToolDisplayName, msg.ToolDetail, msg.ToolStatus, msg.ToolStartedAt, msg.ToolElapsed))
		for _, r := range rows {
			body += "\n " + r
		}
		if expanded {
			body += "\n" + fitBlock(width, m.renderToolExpansion(msg))
		}
		return body
	}

	row := m.formatToolContentAt(width, msg.ToolDisplayName, msg.ToolDetail, msg.ToolStatus, msg.ToolStartedAt, msg.ToolElapsed)
	body := caret + style.Render(row)

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
// message: name, status, the untruncated detail, and the raw input JSON.
//
// This is where the short tag belongs (Task 3c): it is the pointer into
// the Logs tab, so it rides on the record rather than on the summary row
// the user scans — beside a duration it read as a color code.
func (m ChatModel) renderToolExpansion(msg ChatMessage) string {
	var b strings.Builder
	status := string(msg.ToolStatus)
	if msg.ToolElapsed > 0 {
		status += " · " + formatElapsed(msg.ToolElapsed)
	} // The box hangs two columns in, under the row's own text (the caret
	// and its space), so the record reads as that row's detail rather
	// than a block floating a column to the right of it.
	head := "  ┌─ " + msg.ToolDisplayName + " · " + status
	if tag := shortToolTag(msg.ID); tag != "" {
		head += " · " + tag
	}
	b.WriteString(ToolTimeStyle.Render(head))
	if msg.ToolDetail != "" {
		b.WriteString("\n" + ToolTimeStyle.Render("  │  detail: ") + msg.ToolDetail)
	}
	if msg.ToolInputJSON != "" {
		for _, line := range prettyInputJSON(msg.ToolInputJSON) {
			b.WriteString("\n" + ToolTimeStyle.Render("  │  "+line))
		}
	}
	b.WriteString("\n" + ToolTimeStyle.Render("  └─ esc to close"))
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
