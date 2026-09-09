package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	"testing"
)

// upKey/downKey build the arrow-key messages for the history tests.
func upKey() tea.KeyMsg   { return tea.KeyMsg{Type: tea.KeyUp} }
func downKey() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyDown} }

// submit sends the composer content through the real submit path.
func submit(t *testing.T, m ChatModel, text string) ChatModel {
	t.Helper()
	m.textarea.SetValue(text)
	m2, _ := m.doSubmit()
	return m2
}

// sendUpdate routes a key through Update and returns the chat model.
func sendUpdate(m ChatModel, key tea.KeyMsg) ChatModel {
	mm, _ := m.Update(key)
	return mm.(ChatModel)
}

// TestInputHistoryRecall pins goal 0.3.29 Task 6: ↑/↓ recall submitted
// messages shell-style, with draft preservation.
func TestInputHistoryRecall(t *testing.T) {
	m := NewChatModel()
	m.focused = true

	for _, s := range []string{"first prompt", "second prompt", "third prompt"} {
		m = submit(t, m, s)
	}

	// ↑ walks backwards from the newest.
	m = sendUpdate(m, upKey())
	if got := m.textarea.Value(); got != "third prompt" {
		t.Fatalf("first ↑ = %q, want newest submission", got)
	}
	m = sendUpdate(m, upKey())
	if got := m.textarea.Value(); got != "second prompt" {
		t.Fatalf("second ↑ = %q, want 'second prompt'", got)
	}
	m = sendUpdate(m, upKey())
	if got := m.textarea.Value(); got != "first prompt" {
		t.Fatalf("third ↑ = %q, want 'first prompt'", got)
	}
	// ↑ at the oldest stays put.
	m = sendUpdate(m, upKey())
	if got := m.textarea.Value(); got != "first prompt" {
		t.Fatalf("↑ past oldest = %q, want 'first prompt'", got)
	}

	// ↓ walks forward again, then restores the draft.
	m = sendUpdate(m, downKey())
	if got := m.textarea.Value(); got != "second prompt" {
		t.Fatalf("first ↓ = %q, want 'second prompt'", got)
	}
	m = sendUpdate(m, downKey())
	if got := m.textarea.Value(); got != "third prompt" {
		t.Fatalf("second ↓ = %q, want 'third prompt'", got)
	}
	m = sendUpdate(m, downKey())
	if got := m.textarea.Value(); got != "" {
		t.Fatalf("↓ past newest = %q, want empty draft restored", got)
	}
}

// TestInputHistoryPreservesDraft pins the draft contract: a half-typed
// message survives a history trip and comes back on ↓.
func TestInputHistoryPreservesDraft(t *testing.T) {
	m := NewChatModel()
	m.focused = true
	m = submit(t, m, "first prompt")

	m.textarea.SetValue("draft in progress")
	m = sendUpdate(m, upKey())
	if got := m.textarea.Value(); got != "first prompt" {
		t.Fatalf("↑ = %q, want history entry", got)
	}
	m = sendUpdate(m, downKey())
	if got := m.textarea.Value(); got != "draft in progress" {
		t.Fatalf("↓ = %q, want the preserved draft", got)
	}
}

// TestInputHistoryNavigateModeScrolls pins the mode boundary: in
// navigate mode (unfocused) ↑/↓ keep scrolling semantics — they must
// NOT touch the composer.
func TestInputHistoryNavigateModeScrolls(t *testing.T) {
	m := NewChatModel()
	m.focused = false
	m = submit(t, m, "first prompt")
	m.textarea.SetValue("")

	m = sendUpdate(m, upKey())
	if got := m.textarea.Value(); got != "" {
		t.Fatalf("navigate ↑ changed the composer to %q; arrows must scroll, not recall", got)
	}
	m = sendUpdate(m, downKey())
	if got := m.textarea.Value(); got != "" {
		t.Fatalf("navigate ↓ changed the composer to %q; arrows must scroll, not recall", got)
	}
}

// TestInputHistoryDuplicateCollapses pins the noise rule: resubmitting
// the same text does not stack duplicate entries — ↑ passes through
// each distinct submission once.
func TestInputHistoryDuplicateCollapses(t *testing.T) {
	m := NewChatModel()
	m.focused = true
	m = submit(t, m, "same prompt")
	m = submit(t, m, "same prompt")
	m = submit(t, m, "different")

	if n := len(m.inputHistory); n != 2 {
		t.Fatalf("history = %d entries, want 2 (duplicate collapsed)", n)
	}
	m = sendUpdate(m, upKey())
	if got := m.textarea.Value(); got != "different" {
		t.Fatalf("↑ = %q", got)
	}
	m = sendUpdate(m, upKey())
	if got := m.textarea.Value(); got != "same prompt" {
		t.Fatalf("↑↑ = %q, want the single collapsed entry", got)
	}
}

// TestInputHistorySlashCommands pins that slash commands are recorded
// too (re-running /limit or /model is a primary recall use).
func TestInputHistorySlashCommands(t *testing.T) {
	m := NewChatModel()
	m.focused = true
	m = submit(t, m, "/limit 500")

	m = sendUpdate(m, upKey())
	if got := m.textarea.Value(); !strings.HasPrefix(got, "/limit") {
		t.Fatalf("↑ = %q, want the slash command recalled", got)
	}
}
