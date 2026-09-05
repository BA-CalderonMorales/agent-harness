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

// renderTurnBlock renders tools + response as one block: the Agent
// header, then the bubble — prose and tool rows interleaved in the
// order they actually happened, inside the border.
func (m ChatModel) renderTurnBlock(msgs []ChatMessage, i, j int, collapsed bool) (string, int, []clickRef) {
	assistant := msgs[j]

	// Legacy data carries no segmentation: the run nests above the
	// whole answer — never dropped.
	parts := assistant.Parts
	if len(parts) == 0 {
		parts = make([]TurnPart, 0, j-i+1)
		for k := i; k < j; k++ {
			parts = append(parts, TurnPart{ToolID: msgs[k].ID})
		}
		if strings.TrimSpace(assistant.Content) != "" {
			parts = append(parts, TurnPart{Text: assistant.Content})
		}
	}

	// Tool rows resolve by part ID; a row renders through the collapse
	// machinery so runs merge and expansions open in place. The lookup
	// also reports the rendered height for the click index.
	toolRow := func(id string) (string, bool) {
		for k := i; k < j; k++ {
			if msgs[k].ID != id {
				continue
			}
			row, _ := m.renderCollapsedMessage(msgs, k, collapsed)
			return indentBlock(row), true
		}
		return "", false
	}

	var clicks []clickRef
	b := strings.Builder{}
	b.WriteString(m.renderAssistantHeader(assistant))
	b.WriteString("\n")

	width := m.width - 4
	if width < 1 {
		width = 1
	}
	inner := m.assistantInnerContent(assistant, parts, toolRow)
	bubbles := MessageBubbleAssistant.Width(width).Render(inner)
	b.WriteString(bubbles)

	// Click ranges: walk the rendered bubble rows and match the tool
	// rows by their caret content, which survives the border padding.
	row := 1 // the header row
	lines := strings.Split(bubbles, "\n")
	for _, part := range parts {
		if part.ToolID == "" {
			continue
		}
		for offset, bl := range lines[row:] {
			if strings.Contains(bl, "✓") || strings.Contains(bl, "→") || strings.Contains(bl, "✗") {
				// Count the rows of this tool block: the run continues
				// until a non-tool row (prose or blank).
				end := offset
				for end+1 < len(lines[row:]) && !isBubbleProseRow(lines[row+end+1]) {
					end++
				}
				clicks = append(clicks, clickRef{start: row + offset, lines: end - offset + 1, msgID: part.ToolID})
				row += end + 1
				break
			}
		}
	}

	return b.String(), j + 1, clicks
}

// isBubbleProseRow reports whether a bubble row is prose (markdown
// text, blank padding) rather than a tool row — tool rows carry the
// status glyph or caret.
func isBubbleProseRow(line string) bool {
	trimmed := strings.TrimLeft(line, " │")
	if trimmed == "" {
		return true
	}
	return !strings.ContainsAny(trimmed, "✓→✗▸▾")
}
