package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Tool run collapsing: a long-horizon agent turn fires dozens of
// near-identical tool calls, and the flat renderer's wall of
// "✓ bash ..." lines is the #4 reading-pain. Consecutive finalized tool
// messages of the same tool within one turn collapse into a single count
// line ("✓ bash (3) · read (5) — 12s"). Safety guards (non-negotiable):
// an error (or any non-final status) never folds, a running tool renders
// on its own line, and runs never merge across turn boundaries.

// toolRunIsCollapsible reports whether a tool message can participate in
// a collapsed run: only finalized success/complete states. Errors,
// pending/approval states, and running tools force their line visible.
func toolRunIsCollapsible(msg ChatMessage) bool {
	if msg.Role != "tool" {
		return false
	}
	return msg.ToolStatus == ToolStatusSuccess || msg.ToolStatus == ToolStatusComplete
}

// ToggleToolsCollapsed flips the tool-run collapse rendering ('t' key in
// normal mode). Collapsed by default: the wall is the pain, not the
// collapse.
func (m *ChatModel) ToggleToolsCollapsed() {
	m.toolsCollapsed = !m.toolsCollapsed
	m.refreshViewportWithFollow(false)
}

// ToolsCollapsed reports the current collapse state (for tests).
func (m ChatModel) ToolsCollapsed() bool {
	return m.toolsCollapsed
}

// appendCollapsedMessage renders one message, merging a contiguous run
// of finalized same-tool messages (same turn) into a single count line.
// It advances over the consumed messages and returns the new index.
func (m ChatModel) appendCollapsedMessage(content *strings.Builder, msgs []ChatMessage, i int, collapsed bool) int {
	_, next, _ := m.appendCollapsedMessageTracked(content, msgs, i, collapsed)
	return next
}

// blockClick locates the rows of a rendered block that resolve a mouse
// click back to the message, relative to the block's first row. Tools
// map their whole block; the assistant maps only its reasoning rows
// (preview line, or the expanded reasoning frame). Zero lines means the
// block has no click target.
type blockClick struct {
	start int
	lines int
}

// appendCollapsedMessageTracked is appendCollapsedMessage with the
// rendered block returned so the viewport line index can map clicks
// back to messages; the blockClick reports which rows are clickable.
// The returned index is absolute: the caller assigns it straight back
// into the loop cursor.
func (m ChatModel) appendCollapsedMessageTracked(content *strings.Builder, msgs []ChatMessage, i int, collapsed bool) (string, int, blockClick) {
	msg := msgs[i]

	if !collapsed || !toolRunIsCollapsible(msg) {
		return m.appendPlainMessageTracked(content, msgs, i)
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
				return m.appendPlainMessageTracked(content, msgs, i)
			}
		}
	}

	if j-i == 1 {
		return m.appendPlainMessageTracked(content, msgs, i)
	}

	rendered := m.renderToolRun(msgs[i:j])
	content.WriteString(rendered)
	content.WriteString("\n\n")
	lines := strings.Count(rendered, "\n") + 1
	return rendered, j, blockClick{start: 0, lines: lines}
}

// appendPlainMessageTracked renders one message with no run merging and
// reports its clickable rows: the whole block for tools, the reasoning
// rows for assistant messages, nothing otherwise.
func (m ChatModel) appendPlainMessageTracked(content *strings.Builder, msgs []ChatMessage, i int) (string, int, blockClick) {
	msg := msgs[i]
	rendered := m.renderMessage(msg)
	content.WriteString(rendered)
	content.WriteString("\n\n")
	lines := strings.Count(rendered, "\n") + 1

	if msg.IsTool {
		return rendered, i + 1, blockClick{start: 0, lines: lines}
	}
	if msg.Role == "assistant" {
		if start, rows, ok := m.assistantReasoningRows(msg); ok {
			return rendered, i + 1, blockClick{start: start, lines: rows}
		}
	}
	return rendered, i + 1, blockClick{}
}

// renderToolRun renders a collapsed run as one structured record, the
// same shape as a single tool line with the tool column carrying the
// per-tool counts: "01:20:03 ✓ bash ×3 · read ×5   12.3s". The span
// comes from the first and last message timestamps.
func (m ChatModel) renderToolRun(run []ChatMessage) string {
	counts := make(map[string]int)
	for _, msg := range run {
		counts[msg.ToolDisplayName]++
	}
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)

	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%s ×%d", name, counts[name]))
	}

	first := run[0]
	// Span: first start to last settle. Messages carry per-call elapsed
	// times when the live path filled them; tests (and legacy data) only
	// set timestamps, so fall back to the timestamp span.
	start := first.ToolStartedAt
	if start.IsZero() {
		start = first.Timestamp
	}
	end := run[len(run)-1].Timestamp
	if last := run[len(run)-1]; !last.ToolStartedAt.IsZero() && last.ToolElapsed > 0 {
		end = last.ToolStartedAt.Add(last.ToolElapsed)
	}
	span := end.Sub(start)

	detail := strings.Join(parts, " · ")
	// Right-align the duration at the terminal edge with display-width
	// math: rune-byte and ANSI-byte lengths would push the column off
	// the edge. The expand caret opens the line (click toggles the
	// record), and its two columns count toward the left width.
	left := fmt.Sprintf("%s %s %s",
		ToolTimeStyle.Render(start.Format("15:04:05")),
		ToolDoneStyle.Render("✓"),
		detail,
	)
	left = ToolDoneStyle.Render(expandCaret(false)) + " " + left
	dur := ToolTimeStyle.Render(formatElapsed(span))
	pad := m.width - lipgloss.Width(left) - lipgloss.Width(dur) - 2
	if pad < 2 {
		pad = 2
	}
	return left + strings.Repeat(" ", pad) + dur
}
