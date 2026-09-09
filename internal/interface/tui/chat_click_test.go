package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// newClickTestModel builds a chat model sized like the live view, with
// the viewport pinned so click math is deterministic.
func newClickTestModel(t *testing.T) ChatModel {
	t.Helper()
	m := NewChatModel()
	m.width = 120
	m.viewport.Width = 120
	m.viewport.Height = 20
	m.height = 40
	return m
}

// mouseClickAt drives a left-press through the real Update path at the
// given viewport content row.
func mouseClickAt(m ChatModel, row int) ChatModel {
	msg := tea.MouseMsg{
		X:      10,
		Y:      row + viewportTopOffset,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}
	next, _ := m.Update(msg)
	return next.(ChatModel)
}

// TestToolLineClickMapsToMessage pins the tool-line click mapping: the
// tool's whole rendered block resolves back to its message ID.
func TestToolLineClickMapsToMessage(t *testing.T) {
	m := newClickTestModel(t)
	m.AddToolMessage("bash", "bash", "01:20:03 ✓ bash echo hi")
	if len(m.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(m.messages))
	}
	m.refreshViewportWithFollow(true)

	if id := m.expandableMessageAtRow(0); id != m.messages[0].ID {
		t.Fatalf("row 0 resolved to %q, want %q", id, m.messages[0].ID)
	}
}

// TestReasoningPreviewClickExpands pins click-to-expand parity: the
// reasoning preview line inside the streaming assistant bubble resolves
// to the assistant message, and a second click folds it back.
func TestReasoningPreviewClickExpands(t *testing.T) {
	m := newClickTestModel(t)
	model, _ := m.Update(AgentStartMsg{Timestamp: time.Now()})
	m = model.(ChatModel)
	m.SetThinkingText("I should inspect the repo layout first, then decide which files matter")
	m.updateOrCreateStreamingMessage("")
	// Streaming paints batch onto the turn timer's tick; the test
	// paints explicitly before reading the viewport.
	m.refreshViewportWithFollow(true)

	// The preview line is the row the index must map. Find it by
	// content instead of trusting the layout math.
	content := m.viewport.View()
	row := -1
	for i, line := range strings.Split(content, "\n") {
		if strings.Contains(line, "repo layout first") {
			row = i
			break
		}
	}
	if row < 0 {
		t.Fatalf("reasoning preview not rendered:\n%s", content)
	}
	if id := m.expandableMessageAtRow(row); id != m.messages[0].ID {
		t.Fatalf("preview row %d resolved to %q, want %q", row, id, m.messages[0].ID)
	}

	// Click the preview row: expands. Click again: folds.
	m = mouseClickAt(m, row)
	if m.expandedMessageID != m.messages[0].ID {
		t.Fatalf("click on preview row did not expand: %q", m.expandedMessageID)
	}
	m = mouseClickAt(m, row)
	if m.expandedMessageID != "" {
		t.Fatalf("second click did not fold: %q", m.expandedMessageID)
	}
}

// TestAnswerRowsNotClickable pins the boundary: only reasoning rows map
// to the assistant message, never the answer bubble itself.
func TestAnswerRowsNotClickable(t *testing.T) {
	m := newClickTestModel(t)
	m.AddMessage("assistant", "A finished answer in the bubble")
	m.refreshViewportWithFollow(true)

	content := m.viewport.View()
	for i, line := range strings.Split(content, "\n") {
		if strings.Contains(line, "finished answer") {
			if id := m.expandableMessageAtRow(i); id != "" {
				t.Fatalf("answer row %d wrongly resolved to %q", i, id)
			}
			return
		}
	}
	t.Fatalf("answer text not rendered:\n%s", content)
}

// TestCollapsedAffordanceGlyphs pins the discovery affordance: folded
// records carry the ▸ caret, expanded ones ▾.
func TestCollapsedAffordanceGlyphs(t *testing.T) {
	m := newClickTestModel(t)
	m.AddToolMessage("bash", "bash", "echo hi")
	m.refreshViewportWithFollow(true)

	if got := m.viewport.View(); !strings.Contains(got, "▸") {
		t.Fatalf("collapsed tool line missing ▸ caret:\n%s", got)
	}

	m.ExpandLatestRecord()
	if got := m.viewport.View(); !strings.Contains(got, "▾") {
		t.Fatalf("expanded tool line missing ▾ caret:\n%s", got)
	}
}

// TestToolRunClickUnfoldsRun pins run-line behavior: clicking a collapsed
// run unfolds it message-by-message so the expanded record is visible.
func TestToolRunClickUnfoldsRun(t *testing.T) {
	m := newClickTestModel(t)
	model, _ := m.Update(AgentStartMsg{Timestamp: time.Now()})
	m = model.(ChatModel)
	for i := 0; i < 3; i++ {
		// Distinct IDs per call, assigned up front: the streaming
		// assistant's parts carry the tool IDs (live-visibility), so
		// re-IDing messages after the fact orphans the parts (the old
		// rename-the-last-message trick is gone with materialization).
		id := fmt.Sprintf("t%d", i)
		model, _ = m.Update(AgentToolStartMsg{
			ToolID: id, ToolName: "bash", DisplayName: "bash",
			Input: map[string]any{"command": "echo hi"},
		})
		m = model.(ChatModel)
		model, _ = m.Update(AgentToolDoneMsg{ToolID: id, Success: true})
		m = model.(ChatModel)
	}
	m.refreshViewportWithFollow(true)

	if !strings.Contains(m.viewport.View(), "✓ Shell") || !strings.Contains(m.viewport.View(), "$ echo hi") {
		t.Fatalf("run not collapsed to a header + sub-rows:\n%s", m.viewport.View())
	}

	// Row 1: row 0 is the turn's Agent header (live-visibility
	// materializes the turn block), so the run header lives at row 1.
	m = mouseClickAt(m, 1)
	if m.expandedMessageID == "" {
		t.Fatal("click on run line did not expand")
	}
	if got := m.viewport.View(); strings.Contains(got, "✓ Shell") && !strings.Contains(got, "esc to close") {
		t.Fatalf("expanded run still renders collapsed:\n%s", got)
	}
	if !strings.Contains(m.viewport.View(), "esc to close") {
		t.Fatalf("expanded record missing frame:\n%s", m.viewport.View())
	}
}
