package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// driveBurst feeds a tool burst through the real ChatModel update path
// (AgentStartMsg + start/done pairs) and returns the rendered content.
func driveBurst(t *testing.T, pairs []struct {
	name    string
	status  ToolStatus
	running bool // done without a DoneMsg: stays running
}) string {
	t.Helper()
	m := NewChatModel()
	m.width = 120
	m.height = 40
	model, _ := m.Update(AgentStartMsg{Timestamp: time.Now()})
	m = model.(ChatModel)
	for i, p := range pairs {
		id := fmt.Sprintf("t%d", i)
		model, _ = m.Update(AgentToolStartMsg{
			ToolID: id, ToolName: p.name, DisplayName: p.name,
			Input: map[string]any{"command": fmt.Sprintf("echo %d", i)},
		})
		m = model.(ChatModel)
		if !p.running {
			success := p.status != ToolStatusError
			model, _ = m.Update(AgentToolDoneMsg{ToolID: id, Success: success})
			m = model.(ChatModel)
		}
	}
	m.refreshViewportWithFollow(true)
	return m.viewport.View()
}

func renderedLines(content string) []string {
	var lines []string
	for _, l := range strings.Split(content, "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

func TestToolRunCollapseThreeBashCallsToOneLine(t *testing.T) {
	content := driveBurst(t, []struct {
		name    string
		status  ToolStatus
		running bool
	}{
		{"bash", ToolStatusSuccess, false},
		{"bash", ToolStatusSuccess, false},
		{"bash", ToolStatusSuccess, false},
	})
	lines := renderedLines(content)
	if len(lines) != 1 {
		t.Fatalf("3 bash calls rendered %d lines, want 1 collapsed line:\n%s", len(lines), content)
	}
	if !strings.Contains(lines[0], "bash (3)") {
		t.Fatalf("collapsed line missing count: %q", lines[0])
	}
}

func TestToolRunDistinctNamesDoNotCollapse(t *testing.T) {
	content := driveBurst(t, []struct {
		name    string
		status  ToolStatus
		running bool
	}{
		{"bash", ToolStatusSuccess, false},
		{"read", ToolStatusSuccess, false},
		{"grep", ToolStatusSuccess, false},
	})
	lines := renderedLines(content)
	if len(lines) != 3 {
		t.Fatalf("distinct tools rendered %d lines, want 3:\n%s", len(lines), content)
	}
}

func TestToolRunMixedBurstCollapsesRuns(t *testing.T) {
	content := driveBurst(t, []struct {
		name    string
		status  ToolStatus
		running bool
	}{
		{"bash", ToolStatusSuccess, false},
		{"bash", ToolStatusSuccess, false},
		{"read", ToolStatusSuccess, false},
		{"read", ToolStatusSuccess, false},
		{"read", ToolStatusSuccess, false},
		{"grep", ToolStatusSuccess, false},
	})
	lines := renderedLines(content)
	if len(lines) != 3 {
		t.Fatalf("mixed burst rendered %d lines, want 3 (bash(2) read(3) grep(1)):\n%s", len(lines), content)
	}
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"bash (2)", "read (3)", "grep"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "grep (1)") {
		t.Fatalf("single-call runs render plain, not count lines: %q", joined)
	}
}

func TestToolRunErrorForcesExpandedLine(t *testing.T) {
	content := driveBurst(t, []struct {
		name    string
		status  ToolStatus
		running bool
	}{
		{"bash", ToolStatusSuccess, false},
		{"bash", ToolStatusError, false},
		{"bash", ToolStatusSuccess, false},
	})
	lines := renderedLines(content)
	if len(lines) != 3 {
		t.Fatalf("error in the middle rendered %d lines, want 3 expanded:\n%s", len(lines), content)
	}
}

func TestToolRunApprovalPendingNeverCollapsed(t *testing.T) {
	// A pending (approval-waiting) tool never folds: build the messages
	// directly, because the DoneMsg path would finalize the status.
	m := NewChatModel()
	m.width = 120
	m.height = 40
	m.messages = []ChatMessage{
		{Role: "tool", ToolName: "bash", ToolDisplayName: "bash", ToolStatus: ToolStatusSuccess, Turn: 1, Content: "✓ bash"},
		{Role: "tool", ToolName: "bash", ToolDisplayName: "bash", ToolStatus: ToolStatusPending, Turn: 1, Content: "→ bash"},
		{Role: "tool", ToolName: "bash", ToolDisplayName: "bash", ToolStatus: ToolStatusSuccess, Turn: 1, Content: "✓ bash"},
	}
	m.refreshViewportWithFollow(true)
	lines := renderedLines(m.viewport.View())
	if len(lines) != 3 {
		t.Fatalf("pending tool rendered %d lines, want 3 (pending never folds):\n%s", len(lines), m.viewport.View())
	}
}

func TestToolRunRunningNeverFoldedIntoFinalCount(t *testing.T) {
	content := driveBurst(t, []struct {
		name    string
		status  ToolStatus
		running bool
	}{
		{"bash", ToolStatusSuccess, false},
		{"bash", ToolStatusSuccess, false},
		{"bash", ToolStatusRunning, true},
	})
	lines := renderedLines(content)
	if len(lines) != 2 {
		t.Fatalf("2 done + 1 running rendered %d lines, want 2 (running separate):\n%s", len(lines), content)
	}
	if strings.Contains(lines[0], "(3)") {
		t.Fatalf("running tool folded into the finalized count: %q", lines[0])
	}
}

func TestToolRunTwentyBashCallsCollapseToOneLine(t *testing.T) {
	pairs := make([]struct {
		name    string
		status  ToolStatus
		running bool
	}, 20)
	for i := range pairs {
		pairs[i] = struct {
			name    string
			status  ToolStatus
			running bool
		}{"bash", ToolStatusSuccess, false}
	}
	content := driveBurst(t, pairs)
	lines := renderedLines(content)
	if len(lines) != 1 {
		t.Fatalf("20 bash calls rendered %d lines (L metric), want 1:\n%s", len(lines), content)
	}
}

func TestToolRunCollapseToggles(t *testing.T) {
	m := NewChatModel()
	m.width = 120
	m.height = 40
	model, _ := m.Update(AgentStartMsg{Timestamp: time.Now()})
	m = model.(ChatModel)
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("t%d", i)
		model, _ = m.Update(AgentToolStartMsg{ToolID: id, ToolName: "bash", DisplayName: "bash", Input: map[string]any{"command": "ls"}})
		m = model.(ChatModel)
		model, _ = m.Update(AgentToolDoneMsg{ToolID: id, Success: true})
		m = model.(ChatModel)
	}

	if !m.ToolsCollapsed() {
		t.Fatal("collapse should default to on (the wall is the pain)")
	}
	if n := len(renderedLines(m.viewport.View())); n != 1 {
		t.Fatalf("collapsed render = %d lines, want 1", n)
	}

	m.ToggleToolsCollapsed()
	if m.ToolsCollapsed() {
		t.Fatal("toggle did not expand")
	}
	if n := len(renderedLines(m.viewport.View())); n != 3 {
		t.Fatalf("expanded render = %d lines, want 3", n)
	}

	m.ToggleToolsCollapsed()
	if n := len(renderedLines(m.viewport.View())); n != 1 {
		t.Fatalf("re-collapsed render = %d lines, want 1", n)
	}
}

// TestToolRunNeverMergesAcrossTurns: a second agent turn's bash run must
// not merge into the first turn's count line.
func TestToolRunNeverMergesAcrossTurns(t *testing.T) {
	m := NewChatModel()
	m.width = 120
	m.height = 40
	model, _ := m.Update(AgentStartMsg{Timestamp: time.Now()})
	m = model.(ChatModel)
	for i := 0; i < 2; i++ {
		id := fmt.Sprintf("t%d", i)
		model, _ = m.Update(AgentToolStartMsg{ToolID: id, ToolName: "bash", DisplayName: "bash", Input: map[string]any{"command": "ls"}})
		m = model.(ChatModel)
		model, _ = m.Update(AgentToolDoneMsg{ToolID: id, Success: true})
		m = model.(ChatModel)
	}
	model, _ = m.Update(AgentDoneMsg{FullResponse: "done one"})
	m = model.(ChatModel)

	model, _ = m.Update(AgentStartMsg{Timestamp: time.Now()})
	m = model.(ChatModel)
	for i := 0; i < 2; i++ {
		id := fmt.Sprintf("u%d", i)
		model, _ = m.Update(AgentToolStartMsg{ToolID: id, ToolName: "bash", DisplayName: "bash", Input: map[string]any{"command": "ls"}})
		m = model.(ChatModel)
		model, _ = m.Update(AgentToolDoneMsg{ToolID: id, Success: true})
		m = model.(ChatModel)
	}

	lines := renderedLines(m.viewport.View())
	joined := strings.Join(lines, "\n")
	if strings.Contains(joined, "bash (4)") {
		t.Fatalf("runs merged across turns: %q", joined)
	}
	if !strings.Contains(joined, "bash (2)") {
		t.Fatalf("per-turn runs missing: %q", joined)
	}
}

// TestToolRunElapsedShownForLongRuns: the collapsed line carries the run
// span when it is a second or longer.
func TestToolRunElapsedShownForLongRuns(t *testing.T) {
	m := NewChatModel()
	m.width = 120
	m.height = 40
	start := time.Now().Add(-5 * time.Second)
	m.messages = []ChatMessage{
		{Role: "tool", ToolName: "bash", ToolDisplayName: "bash", ToolStatus: ToolStatusSuccess, Turn: 1, Timestamp: start},
		{Role: "tool", ToolName: "bash", ToolDisplayName: "bash", ToolStatus: ToolStatusSuccess, Turn: 1, Timestamp: start.Add(3 * time.Second)},
	}
	m.refreshViewportWithFollow(true)
	line := renderedLines(m.viewport.View())[0]
	if !strings.Contains(line, "3.0s") {
		t.Fatalf("elapsed span missing from collapsed line: %q", line)
	}
}
