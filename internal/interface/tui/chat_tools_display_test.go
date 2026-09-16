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

// TestToolRowDurationFlushRight pins the row's geometry: the duration is
// the row's last visible token and it ends exactly on the row's width
// budget, so a column of calls lines up. The old math added the short
// tag's width twice plus an 8-column fudge, which stranded the duration
// ~13 columns short of the edge — the loudest thing on the row (live
// dogfood). The tag itself is deliberately absent: beside a duration a
// `#d82b` reads as a color code, so it moved to the expanded record and
// the collapsed group header stopped carrying it too.
func TestToolRowDurationFlushRight(t *testing.T) {
	m := NewChatModel()
	start := time.Date(2026, 9, 7, 12, 34, 56, 0, time.UTC)
	for _, width := range []int{40, 48, 60, 80, 120} {
		row := m.formatToolContentAt(width, "Shell", "go test ./... 2>&1 | tail -40", ToolStatusSuccess, start, 2*time.Second)
		plain := strings.TrimRight(ansiStrip(row), " ")
		if !strings.HasSuffix(plain, "2.0s") {
			t.Fatalf("width %d: duration is not the row's last visible token: %q", width, plain)
		}
		if got := lipgloss.Width(plain); got != width {
			t.Fatalf("width %d: row is %d columns, want exactly %d: %q", width, got, width, plain)
		}
		if strings.Contains(plain, "#") {
			t.Fatalf("width %d: the short tag must not ride the summary row: %q", width, plain)
		}
	}

	// A running call keeps the same shape: the ellipsis sits in the
	// duration column, so settling a call does not reflow the list.
	running := m.formatToolContentAt(80, "bash", "go test ./...", ToolStatusRunning, start, 0)
	if plain := strings.TrimRight(ansiStrip(running), " "); !strings.HasSuffix(plain, "…") {
		t.Fatalf("running row lost its duration-column marker: %q", plain)
	}
}

// TestDurationRightAlignedConsistently: the rendered transcript row ends
// flush on the pane's right edge, not a few columns short of it, and the
// viewport shows the duration as the row's last token.
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
		t.Fatalf("duration missing from tool row: %q", line)
	}
	last := strings.TrimRight(ansiStrip(line), " ")
	if !strings.HasSuffix(last, "2.0s") {
		t.Fatalf("duration is not the row's last visible token: %q", last)
	}
	// The transcript spans the frame's inner width, so a standalone tool
	// row's duration lands on the right edge: only the pane's own frame
	// border sits beyond it.
	if gap := lipgloss.Width(line) - lipgloss.Width(last); gap > 1 {
		t.Fatalf("duration is %d columns short of the pane edge: %q", gap, last)
	}
}

// ansiStrip removes SGR sequences so suffix checks read the visible
// text, not the styling bytes.
func ansiStrip(s string) string { return ansi.Strip(s) }
