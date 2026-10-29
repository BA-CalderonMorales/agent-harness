package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestComposerExplainsConnectionAndActions(t *testing.T) {
	for _, tc := range []struct{ provider, want string }{
		{"lemonade", "local: Lemonade"}, {"flm", "local: FastFlowLM (direct)"},
		{"openrouter", "hosted: OpenRouter"},
	} {
		m := NewChatModel()
		m.width, m.focused = 180, true
		m.provider, m.model, m.agentMode = tc.provider, "test-model", "manual"
		line := ansi.Strip(m.renderModeLine())
		for _, want := range []string{tc.want, "manual: ask", "Enter send", "Alt+Enter newline"} {
			if !strings.Contains(line, want) {
				t.Errorf("%s missing %q: %s", tc.provider, want, line)
			}
		}
	}
}

func TestComposerKeepsStopHintWhenUnfocusedAndCrowded(t *testing.T) {
	for _, width := range []int{32, 40, 60, 80, 120} {
		m := NewChatModel()
		m.width, m.focused, m.thinking = width, false, true
		m.provider, m.model, m.persona = "openrouter", strings.Repeat("long-model-", 8), "developer"
		line := m.renderModeLine()
		if !strings.Contains(ansi.Strip(line), `"Esc" to stop`) {
			t.Fatalf("width %d hides cancellation: %s", width, line)
		}
		if strings.Contains(line, "\n") || ansi.StringWidth(line) > m.composerWidth() {
			t.Fatalf("width %d overflow: %q", width, line)
		}
	}
}

func TestToolSummaryFitsNarrowPanesAndKeepsMultilineInput(t *testing.T) {
	m := NewChatModel()
	started := time.Date(2026, 10, 29, 12, 30, 0, 0, time.UTC)
	for _, status := range []ToolStatus{ToolStatusPending, ToolStatusRunning, ToolStatusSuccess, ToolStatusError} {
		for _, width := range []int{1, 8, 12, 20, 32, 40, 60, 80, 120} {
			row := m.formatToolContentAt(width, "Read File", "names.py\n  test_names.py", status, started, 120*time.Millisecond)
			if strings.Contains(row, "\n") || ansi.StringWidth(row) > width {
				t.Fatalf("width %d status %s overflow: %q", width, status, row)
			}
			if width >= 32 && !strings.Contains(row, "names.py") {
				t.Errorf("width %d lost action: %s", width, row)
			}
			if status == ToolStatusPending && strings.Contains(row, "✓") {
				t.Errorf("pending marked successful: %s", row)
			}
		}
	}
	command := "printf first\nprintf second"
	m.AddOrUpdateToolMessage("call", "bash", "Shell", command, ToolStatusRunning)
	if m.messages[0].ToolDetail != command {
		t.Fatal("summary formatting changed the underlying command")
	}
}

func TestToolGroupsHaveSeparateHeaders(t *testing.T) {
	m := NewChatModel()
	run := []ChatMessage{
		{Role: "tool", ToolName: "read", ToolDetail: "names.py", ToolStatus: ToolStatusSuccess},
		{Role: "tool", ToolName: "bash", ToolDetail: "python -m unittest -v", ToolStatus: ToolStatusSuccess},
	}
	view := ansi.Strip(m.renderToolRunAt(run, 80))
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "names.py") && strings.Contains(line, "Shell") {
			t.Fatalf("group header runs into previous call: %s", view)
		}
	}
}
