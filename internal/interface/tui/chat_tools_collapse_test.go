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
	m.viewport.Width = 120
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
	// Codex-style grouping (Task 4): one "Shell" header with each call
	// as an indented sub-list row beneath it — 4 lines, not a count line.
	if len(lines) != 4 {
		t.Fatalf("3 bash calls rendered %d lines, want header + 3 sub-rows:\n%s", len(lines), content)
	}
	if !strings.Contains(lines[0], "Shell") {
		t.Fatalf("group header missing display name: %q", lines[0])
	}
	if n := strings.Count(content, "$ echo"); n != 3 {
		t.Fatalf("sub-list rows missing ($ echo ×3), got %d:\n%s", n, content)
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
	// bash(2) + read(3) + grep(1): 3 group headers + 6 sub-list rows
	// (grep's detail falls back to its name — 8 non-blank lines total).
	if len(lines) != 8 {
		t.Fatalf("mixed burst rendered %d lines, want 3 headers + 5 sub-rows (grep detail = its name):\n%s", len(lines), content)
	}
	joined := strings.Join(lines, "\n")
	// Group headers carry display names for multi-call runs; the
	// single grep call renders its own line (single calls never group),
	// and sub-rows carry the details.
	for _, want := range []string{"Shell", "Read File", "grep", "$ echo"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in:\n%s", want, joined)
		}
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
	m.viewport.Width = 120
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
	// 2 done render as one collapsed group (header + 2 sub-rows); the
	// running call stays on its own line — never folded into the group.
	if len(lines) != 4 {
		t.Fatalf("2 done + 1 running rendered %d lines, want 3 group rows + running line:\n%s", len(lines), content)
	}
	if !strings.Contains(lines[3], "→") {
		t.Fatalf("running tool line missing running glyph:\n%s", content)
	}
	if strings.Contains(lines[0], "(3)") {
		t.Fatalf("running tool folded into the finalized count: %q", lines[0])
	}
}

func TestToolRunTwentyBashCallsCollapseToOneHeader(t *testing.T) {
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
	// 21 rendered rows need a viewport that fits them; resize() sizes
	// the viewport from the model height (a bare m.height doesn't).
	m := NewChatModel()
	m.width = 120
	m.resize(120, 60)
	model, _ := m.Update(AgentStartMsg{Timestamp: time.Now()})
	m = model.(ChatModel)
	for i, p := range pairs {
		id := fmt.Sprintf("t%d", i)
		model, _ = m.Update(AgentToolStartMsg{
			ToolID: id, ToolName: p.name, DisplayName: p.name,
			Input: map[string]any{"command": fmt.Sprintf("echo %d", i)},
		})
		m = model.(ChatModel)
		model, _ = m.Update(AgentToolDoneMsg{ToolID: id, Success: p.status != ToolStatusError && !p.running})
		m = model.(ChatModel)
	}
	m.refreshViewportWithFollow(true)
	content := m.viewport.View()
	// With no clipping, assert the full structure: one header for the
	// whole run, every call as an indented sub-list row.
	if n := strings.Count(content, "✓ Shell"); n != 1 {
		t.Fatalf("expected exactly one Shell header, got %d:\n%s", n, content)
	}
	if n := strings.Count(content, "$ echo"); n != 20 {
		t.Fatalf("expected 20 sub-list rows, got %d:\n%s", n, content)
	}
}

func TestToolRunCollapseToggles(t *testing.T) {
	m := NewChatModel()
	m.width = 120
	m.viewport.Width = 120
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
	// Collapsed: one Shell header + 3 indented sub-list rows.
	if n := len(renderedLines(m.viewport.View())); n != 4 {
		t.Fatalf("collapsed render = %d lines, want 1 header + 3 sub-rows", n)
	}

	m.ToggleToolsCollapsed()
	if m.ToolsCollapsed() {
		t.Fatal("toggle did not expand")
	}
	if n := len(renderedLines(m.viewport.View())); n != 3 {
		t.Fatalf("expanded render = %d lines, want 3", n)
	}

	m.ToggleToolsCollapsed()
	if n := len(renderedLines(m.viewport.View())); n != 4 {
		t.Fatalf("re-collapsed render = %d lines, want 1 header + 3 sub-rows", n)
	}
}

// TestToolRunNeverMergesAcrossTurns: a second agent turn's bash run must
// not merge into the first turn's count line.
func TestToolRunNeverMergesAcrossTurns(t *testing.T) {
	m := NewChatModel()
	m.width = 120
	m.viewport.Width = 120
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
	// Each turn renders its own group; the two turns' calls must not
	// share one sub-list. Two headers total (one per turn), 4 sub-rows.
	headers := strings.Count(joined, "✓ Shell")
	if headers != 2 {
		t.Fatalf("runs merged across turns: %d headers, want 2:\n%s", headers, joined)
	}
	if n := strings.Count(joined, "$ ls"); n != 4 {
		t.Fatalf("per-turn sub-rows missing: %d '$ ls' rows, want 4:\n%s", n, joined)
	}
}

// TestToolRunElapsedShownForLongRuns: the collapsed line carries the run
// span when it is a second or longer.
func TestToolRunElapsedShownForLongRuns(t *testing.T) {
	m := NewChatModel()
	m.width = 120
	m.viewport.Width = 120
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
