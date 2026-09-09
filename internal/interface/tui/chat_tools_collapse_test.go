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
	// as an indented sub-list row beneath it (blank separators between
	// sub-rows are bubble-bordered, so they count as lines). The
	// live-visibility fix materializes the turn bubble on tool start,
	// so the block carries the Agent header first: Agent + header +
	// 3 sub-rows + 2 separators = 7.
	if len(lines) != 7 {
		t.Fatalf("3 bash calls rendered %d lines, want Agent + header + 3 sub-rows + 2 separators:\n%s", len(lines), content)
	}
	if !strings.Contains(lines[1], "Shell") {
		t.Fatalf("group header missing display name: %q", lines[1])
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
	// 3 distinct tools render as 3 plain rows (single calls never
	// group) under the turn's Agent header: 4 lines.
	if len(lines) != 4 {
		t.Fatalf("distinct tools rendered %d lines, want Agent header + 3 rows:\n%s", len(lines), content)
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
	// bash(2) + read(3) + grep(1): Agent + 2 group headers + 5 sub-rows
	// + 4 sub-row separators + the single grep row — 15 non-blank lines.
	if len(lines) != 15 {
		t.Fatalf("mixed burst rendered %d lines, want Agent + 2 headers + 5 sub-rows + 4 separators + grep row:\n%s", len(lines), content)
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
	if len(lines) != 7 {
		t.Fatalf("error in the middle rendered %d lines, want Agent + header + 3 expanded + 2 separators:\n%s", len(lines), content)
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
		t.Fatalf("pending tool rendered %d lines, want 3 expanded (pending never folds):\n%s", len(lines), m.viewport.View())
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
	// running call stays on its own line — never folded into the
	// group. With the live-visibility fix the block carries the Agent
	// header first: Agent + group header + 2 sub-rows + 1 separator +
	// running = 7.
	if len(lines) != 7 {
		t.Fatalf("2 done + 1 running rendered %d lines, want Agent + header + 2 sub-rows + sep + running:\n%s", len(lines), content)
	}
	if !strings.Contains(lines[6], "→") {
		t.Fatalf("running tool line missing running glyph:\n%s", content)
	}
	if strings.Contains(lines[1], "(3)") {
		t.Fatalf("running tool folded into the finalized count: %q", lines[1])
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
	// Collapsed: Agent header + one Shell header + 3 indented sub-list
	// rows + 2 sub-row separators (blank bubble-bordered lines count).
	if n := len(renderedLines(m.viewport.View())); n != 7 {
		t.Fatalf("collapsed render = %d lines, want Agent + header + 3 sub-rows + 2 separators", n)
	}

	m.ToggleToolsCollapsed()
	if m.ToolsCollapsed() {
		t.Fatal("toggle did not expand")
	}
	if n := len(renderedLines(m.viewport.View())); n != 7 {
		t.Fatalf("expanded render = %d lines, want Agent + header + 3 sub-rows + 2 separators", n)
	}

	m.ToggleToolsCollapsed()
	if n := len(renderedLines(m.viewport.View())); n != 7 {
		t.Fatalf("re-collapsed render = %d lines, want Agent + header + 3 sub-rows + 2 separators", n)
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

// TestBurstSpanningInterleavedProseIsOneGroup pins goal 0.3.29 Task 3a:
// bash + prose + bash within one turn renders ONE Shell header with two
// sub-rows. Codex groups the whole burst per class; the old contiguous
// scan split at the interleaved narration (or a live-block boundary),
// rendering two "▸ ✓ Shell" headers for one continuous shell burst.
func TestBurstSpanningInterleavedProseIsOneGroup(t *testing.T) {
	m := NewChatModel()
	m.width = 120
	m.viewport.Width = 120
	m.height = 40
	model, _ := m.Update(AgentStartMsg{Timestamp: time.Now()})
	m = model.(ChatModel)

	// Tool 1, then interleaved prose (the model narrates between calls),
	// then tool 2 — all within one turn.
	model, _ = m.Update(AgentToolStartMsg{ToolID: "t0", ToolName: "bash", DisplayName: "Bash", Input: map[string]any{"command": "echo one"}})
	m = model.(ChatModel)
	model, _ = m.Update(AgentToolDoneMsg{ToolID: "t0", Success: true})
	m = model.(ChatModel)

	model, _ = m.Update(AgentChunkMsg{Text: "Now checking the second path.", Timestamp: time.Now()})
	m = model.(ChatModel)

	model, _ = m.Update(AgentToolStartMsg{ToolID: "t1", ToolName: "bash", DisplayName: "Bash", Input: map[string]any{"command": "echo two"}})
	m = model.(ChatModel)
	model, _ = m.Update(AgentToolDoneMsg{ToolID: "t1", Success: true})
	m = model.(ChatModel)

	m.refreshViewportWithFollow(true)
	content := m.viewport.View()

	headers := 0
	for _, l := range renderedLines(content) {
		if strings.Contains(l, "Shell") {
			headers++
		}
	}
	if headers != 1 {
		t.Fatalf("bash+prose+bash rendered %d Shell headers, want 1:\n%s", headers, content)
	}
	if !strings.Contains(content, "$ echo one") || !strings.Contains(content, "$ echo two") {
		t.Fatalf("burst sub-rows missing both commands:\n%s", content)
	}
}

// TestSystemNoteDoesNotSplitBurst pins the live-block boundary case from
// the 0.3.29 dogfood pass: a [Tool loop detected: ...] system note
// landing between two Shell calls must not split the burst into two
// headers (same class-grouping rule as prose, verified through the real
// update path with AgentSystemNoteMsg between the done/start pairs).
func TestSystemNoteDoesNotSplitBurst(t *testing.T) {
	m := NewChatModel()
	m.width = 120
	m.viewport.Width = 120
	m.height = 40
	model, _ := m.Update(AgentStartMsg{Timestamp: time.Now()})
	m = model.(ChatModel)
	model, _ = m.Update(AgentToolStartMsg{ToolID: "t0", ToolName: "bash", DisplayName: "bash", Input: map[string]any{"command": "echo one"}})
	m = model.(ChatModel)
	model, _ = m.Update(AgentToolDoneMsg{ToolID: "t0", Success: true})
	m = model.(ChatModel)
	model, _ = m.Update(AgentSystemNoteMsg{Text: "[Tool loop detected: bash was called 2 times with identical input.]"})
	m = model.(ChatModel)
	model, _ = m.Update(AgentToolStartMsg{ToolID: "t1", ToolName: "bash", DisplayName: "bash", Input: map[string]any{"command": "echo two"}})
	m = model.(ChatModel)
	model, _ = m.Update(AgentToolDoneMsg{ToolID: "t1", Success: true})
	m = model.(ChatModel)
	m.refreshViewportWithFollow(true)
	content := m.viewport.View()
	headers := 0
	for _, l := range renderedLines(content) {
		if strings.Contains(l, "Shell") {
			headers++
		}
	}
	if headers != 1 {
		t.Fatalf("bash+note+bash rendered %d Shell headers, want 1:\n%s", headers, content)
	}
}
