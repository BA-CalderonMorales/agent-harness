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

// appendCollapsedMessageTracked is appendCollapsedMessage with the
// rendered block returned so the viewport line index can map clicks
// back to messages; isToolRender reports whether the block is a tool
// record (clicks only target tool lines).
func (m ChatModel) appendCollapsedMessageTracked(content *strings.Builder, msgs []ChatMessage, i int, collapsed bool) (string, int, bool) {
	msg := msgs[i]

	if !collapsed || !toolRunIsCollapsible(msg) {
		rendered := m.renderMessage(msg)
		content.WriteString(rendered)
		content.WriteString("\n\n")
		return rendered, i + 1, msg.IsTool
	}

	// Gather the contiguous run: same turn, same tool, all final.
	j := i + 1
	for j < len(msgs) && msgs[j].Role == "tool" &&
		msgs[j].Turn == msg.Turn &&
		msgs[j].ToolName == msg.ToolName &&
		toolRunIsCollapsible(msgs[j]) {
		j++
	}

	if j-i == 1 {
		rendered := m.renderMessage(msg)
		content.WriteString(rendered)
		content.WriteString("\n\n")
		return rendered, j, msg.IsTool
	}

	rendered := m.renderToolRun(msgs[i:j])
	content.WriteString(rendered)
	content.WriteString("\n\n")
	return rendered, j, true
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
	// the edge.
	left := fmt.Sprintf("%s %s %s",
		ToolTimeStyle.Render(start.Format("15:04:05")),
		ToolDoneStyle.Render("✓"),
		detail,
	)
	dur := ToolTimeStyle.Render(formatElapsed(span))
	pad := m.width - lipgloss.Width(left) - lipgloss.Width(dur) - 2
	if pad < 2 {
		pad = 2
	}
	return left + strings.Repeat(" ", pad) + dur
}
