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
	// Right-aligned: the duration is the last visible token on the row
	// — at most the 2 reserved caret/pad columns of trailing space
	// after it. (Terminal-edge padding is stripped before measuring.)
	trimmed := strings.TrimRight(line, " ")
	if lipgloss.Width(line)-lipgloss.Width(trimmed) > 2 {
		t.Fatalf("duration not right-aligned (more than 2 trailing columns): %q", line)
	}
	if !strings.HasSuffix(ansiStrip(trimmed), "2.0s") {
		t.Fatalf("duration is not the last visible token on the row: %q", trimmed)
	}
}

// ansiStrip removes SGR sequences so suffix checks read the visible
// text, not the styling bytes.
func ansiStrip(s string) string { return ansi.Strip(s) }
