package tui

import "strings"

// Turn grouping: the agent's tool calls belong to its response, not to
// the transcript at large. A turn renders as one block — the Agent
// header, the turn's tool calls nested beneath it (runs merged,
// expansions opening in place), then the answer bubble. A tool run
// that trails the transcript (still streaming, no answer yet) renders
// standalone and is absorbed when the response lands.

// renderCollapsedMessage renders one message with run merging applied,
// pure: no writing, no separators. next is the caller's next index.
func (m ChatModel) renderCollapsedMessage(msgs []ChatMessage, i int, collapsed bool) (string, int) {
	msg := msgs[i]

	if !collapsed || !toolRunIsCollapsible(msg) {
		return m.renderMessage(msg), i + 1
	}

	// Gather the contiguous run: same turn, same tool, all final.
	j := i + 1
	for j < len(msgs) && msgs[j].Role == "tool" &&
		msgs[j].Turn == msg.Turn &&
		msgs[j].ToolName == msg.ToolName &&
		toolRunIsCollapsible(msgs[j]) {
		j++
	}

	// Expanding any member of a run unfolds the run message-by-message
	// so the expanded record has a visible home.
	if j-i > 1 {
		for k := i; k < j; k++ {
			if m.expandedMessageID != "" && m.expandedMessageID == msgs[k].ID {
				return m.renderMessage(msg), i + 1
			}
		}
	}

	if j-i == 1 {
		return m.renderMessage(msg), i + 1
	}

	return m.renderToolRun(msgs[i:j]), j
}

// indentBlock prefixes every line of a rendered block with a single
// space — the nesting step under the Agent header. Line counts never
// change, so click mapping stays honest.
func indentBlock(block string) string {
	lines := strings.Split(block, "\n")
	for i, line := range lines {
		lines[i] = " " + line
	}
	return strings.Join(lines, "\n")
}

// clickRef is a clickable region inside a rendered block: the row
// offset relative to the block's first row, how many rows, and the
// message the rows resolve to.
type clickRef struct {
	start, lines int
	msgID        string
}

// appendTurnGroupTracked renders the next render group starting at i
// and reports the clickable line ranges within the block (row offsets
// relative to the block's first row). next is the loop's next index.
func (m ChatModel) appendTurnGroupTracked(msgs []ChatMessage, i int, collapsed bool) (string, int, []clickRef) {
	msg := msgs[i]

	// Tool messages buffer into the response that follows them.
	if msg.Role == "tool" {
		j := i
		for j < len(msgs) && msgs[j].Role == "tool" {
			j++
		}
		if j < len(msgs) && msgs[j].Role == "assistant" {
			return m.renderTurnBlock(msgs, i, j, collapsed)
		}
		// Trailing run: still streaming, no response yet — standalone,
		// absorbed when the answer lands.
		return m.renderSingleGroup(msgs, i, collapsed)
	}

	return m.renderSingleGroup(msgs, i, collapsed)
}

// renderSingleGroup renders one message through the collapse machinery.
func (m ChatModel) renderSingleGroup(msgs []ChatMessage, i int, collapsed bool) (string, int, []clickRef) {
	rendered, next := m.renderCollapsedMessage(msgs, i, collapsed)
	var clicks []clickRef
	if msgs[i].IsTool {
		clicks = append(clicks, clickRef{start: 0, lines: strings.Count(rendered, "\n") + 1, msgID: msgs[i].ID})
	} else if msgs[i].Role == "assistant" {
		if start, rows, ok := m.assistantReasoningRows(msgs[i]); ok {
			clicks = append(clicks, clickRef{start: start, lines: rows, msgID: msgs[i].ID})
		}
	}
	return rendered, next, clicks
}

// renderTurnBlock renders tools + response as one block: header,
// nested tool lines (one space in), blank, answer. Click ranges map
// each tool's rows and the assistant's reasoning rows.
func (m ChatModel) renderTurnBlock(msgs []ChatMessage, i, j int, collapsed bool) (string, int, []clickRef) {
	assistant := msgs[j]

	var b strings.Builder
	var clicks []clickRef

	b.WriteString(m.renderAssistantHeader(assistant))
	b.WriteString("\n")

	line := 1 // the header row
	for k := i; k < j; {
		part, next := m.renderCollapsedMessage(msgs, k, collapsed)
		lines := strings.Count(part, "\n") + 1
		b.WriteString(indentBlock(part))
		b.WriteString("\n")
		clicks = append(clicks, clickRef{start: line, lines: lines, msgID: msgs[k].ID})
		line += lines + 1
		k = next
	}
	b.WriteString("\n")

	content := m.renderAssistantContent(assistant)
	contentLines := strings.Count(content, "\n") + 1
	b.WriteString(content)
	if start, rows, ok := m.assistantReasoningRows(assistant); ok {
		clicks = append(clicks, clickRef{start: line + start, lines: rows, msgID: assistant.ID})
	}
	_ = contentLines

	return b.String(), j + 1, clicks
}
