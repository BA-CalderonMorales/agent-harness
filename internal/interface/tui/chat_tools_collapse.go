package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"
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
	msg := msgs[i]

	if !collapsed || !toolRunIsCollapsible(msg) {
		content.WriteString(m.renderMessage(msg))
		content.WriteString("\n\n")
		return i + 1
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
		content.WriteString(m.renderMessage(msg))
		content.WriteString("\n\n")
		return j
	}

	content.WriteString(m.renderToolRun(msgs[i:j]))
	content.WriteString("\n\n")
	return j
}

// renderToolRun renders a collapsed run as one status line:
// "✓ bash (3) · read (5) — 12s". The elapsed span comes from the first
// and last message timestamps; a sub-second run omits it.
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
		parts = append(parts, fmt.Sprintf("%s (%d)", name, counts[name]))
	}

	line := "✓ " + strings.Join(parts, " · ")
	span := run[len(run)-1].Timestamp.Sub(run[0].Timestamp)
	if span >= time.Second {
		line += " — " + formatElapsed(span)
	}
	return ToolDoneStyle.Render(line)
}
