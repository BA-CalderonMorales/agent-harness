package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// These tests pin the 0.3.28 codex-style tool-call display (Task 4):
// grouped headers with indented sub-lists, class renames (ls → Shell,
// edit → Update), and todo calls rendered as a visible checklist.

// TestTodoCallRendersChecklist: a todo_write call renders as checkbox
// rows, not a bare tool line.
func TestTodoCallRendersChecklist(t *testing.T) {
	m := NewChatModel()
	m.width = 120
	m.resize(120, 40)

	todos := `{"todos":[{"id":"1","text":"investigate scroll bug","status":"completed"},{"id":"2","text":"fix deferred refresh","status":"in_progress"},{"id":"3","text":"run tests","status":"pending"}]}`
	m.messages = []ChatMessage{
		{ID: "todo-1", Role: "tool", IsTool: true, ToolName: "todo_write",
			ToolDisplayName: "Todo", ToolStatus: ToolStatusSuccess,
			ToolInputJSON: todos, ToolDetail: "Updated 3 todos",
			Timestamp: time.Now(), Turn: 1},
	}
	m.refreshViewportWithFollow(true)
	content := m.viewport.View()
	for _, want := range []string{"✓ investigate scroll bug", "→ fix deferred refresh", "○ run tests"} {
		if !strings.Contains(content, want) {
			t.Fatalf("checklist row missing %q:\n%s", want, content)
		}
	}
}

// TestTodoChecklistNilWithoutInput: a todo call without parseable input
// falls back to the normal detail row, never an empty group.
func TestTodoChecklistNilWithoutInput(t *testing.T) {
	m := NewChatModel()
	m.width = 120
	m.resize(120, 40)
	m.messages = []ChatMessage{
		{ID: "todo-2", Role: "tool", IsTool: true, ToolName: "todo_write",
			ToolDisplayName: "Todo", ToolStatus: ToolStatusSuccess,
			ToolInputJSON: "", ToolDetail: "Updated 2 todos",
			Timestamp: time.Now(), Turn: 1},
	}
	m.refreshViewportWithFollow(true)
	content := m.viewport.View()
	if !strings.Contains(content, "Updated 2 todos") {
		t.Fatalf("fallback detail missing:\n%s", content)
	}
	if strings.Contains(content, "○") {
		t.Fatalf("empty checklist rendered:\n%s", content)
	}
}

// TestLsGroupedUnderShellHeader: consecutive bash+ls+ls calls all group
// under one "Shell" header — ls never gets its own header class.
func TestLsGroupedUnderShellHeader(t *testing.T) {
	m := NewChatModel()
	m.width = 120
	m.resize(120, 40)
	model, _ := m.Update(AgentStartMsg{Timestamp: time.Now()})
	m = model.(ChatModel)
	calls := []struct{ name, cmd string }{
		{"bash", "echo start"},
		{"ls", "ls /tmp"},
		{"ls", "ls /var"},
	}
	for i, c := range calls {
		model, _ = m.Update(AgentToolStartMsg{
			ToolID: string(rune('a' + i)), ToolName: c.name, DisplayName: c.name,
			Input: map[string]any{"command": c.cmd},
		})
		m = model.(ChatModel)
		model, _ = m.Update(AgentToolDoneMsg{ToolID: string(rune('a' + i)), Success: true})
		m = model.(ChatModel)
	}
	m.refreshViewportWithFollow(true)
	content := m.viewport.View()
	if n := strings.Count(content, "Shell"); n != 1 {
		t.Fatalf("bash+ls+ls rendered %d Shell headers, want 1 grouped:\n%s", n, content)
	}
	if !strings.Contains(content, "$ echo start") || !strings.Contains(content, "ls /tmp") || !strings.Contains(content, "ls /var") {
		t.Fatalf("sub-list rows missing:\n%s", content)
	}
}

// TestDurationRightAlignedConsistently: single rows and group headers
// both right-align their duration at the width edge.
func TestDurationRightAlignedConsistently(t *testing.T) {
	m := NewChatModel()
	m.width = 120
	m.resize(120, 40)
	start := time.Now().Add(-2 * time.Second)
	m.messages = []ChatMessage{
		{ID: "t1", Role: "tool", IsTool: true, ToolName: "bash",
			ToolDisplayName: "Shell", ToolStatus: ToolStatusSuccess,
			ToolDetail: "echo hi", Timestamp: start,
			ToolStartedAt: start, ToolElapsed: 2 * time.Second, Turn: 1},
	}
	m.refreshViewportWithFollow(true)
	line := renderedLines(m.viewport.View())[0]
	if !strings.Contains(line, "2.0s") {
		t.Fatalf("duration missing from group header: %q", line)
	}
	// Right-aligned within the row's own render width: the duration
	// (plus the stable short tag, Task 3c — the tag is the new
	// right-most token by design) ends at the row's content edge. The
	// viewport pads bubble-nested rows out to the pane width, so
	// measure the trailing gap against the row's rendered width, not
	// the pane: at most the 2 reserved caret/pad columns of trailing
	// space before the viewport padding.
	trimmed := strings.TrimRight(line, " ")
	contentW := lipgloss.Width(trimmed)
	trail := lipgloss.Width(line) - contentW
	// Trail = (pane − row render width) + at most 2 internal pad cols.
	// The row render width is pane−8−1 (bubble inner minus left
	// border), so the gap can be up to 9+2; assert the tag/duration
	// ends within the row's own budget by checking the gap never
	// exceeds the bubble slack (9) plus the 2 internal pad columns.
	if trail > 11 {
		t.Fatalf("duration not right-aligned (more than bubble slack + 2 trailing columns): %q", line)
	}
	last := strings.TrimRight(ansiStrip(trimmed), " ")
	if !strings.HasSuffix(last, "#5e16") {
		// The tag is the row's last visible token (Task 3c).
		t.Fatalf("tag is not the row's last visible token: %q", last)
	}
}

// ansiStrip removes SGR sequences so suffix checks read the visible
// text, not the styling bytes.
func ansiStrip(s string) string { return ansi.Strip(s) }
