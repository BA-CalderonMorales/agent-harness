package tui

import (
	"strings"
	"testing"
	"time"
)

// These tests pin the 0.3.28 cancel-leak fix: ESC on a thinking turn
// left the streaming assistant message with Thinking=true and the
// streaming pointer set, so the dead turn rendered the animated
// thinking badge with a frozen elapsed time forever — and the next
// turn inherited the stale live-header state.

// startThinkingTurn drives a turn to the state where the assistant
// section has materialized with partial content.
func startThinkingTurn(t *testing.T, content string) ChatModel {
	t.Helper()
	m := NewChatModel()
	m.width = 100
	m.height = 24
	m.resize(100, 24)
	m.SetInput("hi")
	model, _ := m.Update(AgentStartMsg{Timestamp: time.Now()})
	m = model.(ChatModel)
	model, _ = m.Update(AgentChunkMsg{Text: content, Timestamp: time.Now()})
	m = model.(ChatModel)
	// Materialize the placeholder (elapsed >= PlaceholderDelay).
	m.startTime = time.Now().Add(-1500 * time.Millisecond)
	model, _ = m.Update(timerTickMsg{})
	m = model.(ChatModel)
	return m
}

// TestCancelFinalizesPartialAssistantMessage: ESC on a streaming turn
// with content must set Thinking=false on that message — no frozen
// spinner badge.
func TestCancelFinalizesPartialAssistantMessage(t *testing.T) {
	m := startThinkingTurn(t, "partial answer")

	model, _ := m.Update(AgentCancelMsg{})
	m = model.(ChatModel)

	var assistant *ChatMessage
	for i := range m.messages {
		if m.messages[i].Role == "assistant" {
			assistant = &m.messages[i]
		}
	}
	if assistant == nil {
		t.Fatal("assistant message with content must survive cancel, not be dropped")
	}
	if assistant.Thinking {
		t.Fatal("cancelled assistant message still has Thinking=true (frozen spinner)")
	}
	if m.currentStreamingAssistantID != "" {
		t.Fatalf("streaming pointer survived cancel: %q", m.currentStreamingAssistantID)
	}
}

// TestCancelEmptyThinkingDropsPlaceholder: ESC before the first token
// drops the placeholder — no dead thinking bubble.
func TestCancelEmptyThinkingDropsPlaceholder(t *testing.T) {
	m := startThinkingTurn(t, "")

	model, _ := m.Update(AgentCancelMsg{})
	m = model.(ChatModel)

	for _, msg := range m.messages {
		if msg.Role == "assistant" {
			t.Fatal("empty assistant placeholder survived cancel")
		}
	}
}

// TestNewTurnAfterCancelRendersFresh: after a cancel, a new AgentStartMsg
// must produce a fresh placeholder state (zero elapsed, no inherited
// spinner) — and the cancelled message must not render the spinner.
func TestNewTurnAfterCancelRendersFresh(t *testing.T) {
	m := startThinkingTurn(t, "cancelled answer")

	model, _ := m.Update(AgentCancelMsg{})
	m = model.(ChatModel)

	// Start the next turn.
	model, _ = m.Update(AgentStartMsg{Timestamp: time.Now()})
	m = model.(ChatModel)

	if m.turnInterrupted {
		t.Fatal("new AgentStartMsg did not clear turnInterrupted")
	}
	if m.elapsed != 0 {
		t.Fatalf("new turn inherited elapsed %v, want 0", m.elapsed)
	}
	if !m.placeholderPending {
		t.Fatal("new turn did not set placeholderPending (fresh placeholder)")
	}
}

// TestCancelledMessageNotInStreamingCacheSignature: the cancelled
// message's rendered frame must not contain the live thinking badge,
// even when the group cache is consulted.
func TestCancelledMessageNotInStreamingCacheSignature(t *testing.T) {
	m := startThinkingTurn(t, "answer that survived the cancel")

	model, _ := m.Update(AgentCancelMsg{})
	m = model.(ChatModel)

	m.refreshViewportWithFollow(true)
	for i := range m.messages {
		msg := &m.messages[i]
		if msg.Role == "assistant" && strings.Contains(msg.Content, "survived") {
			rendered, _, _ := m.renderSingleGroup(m.messages, i, m.toolsCollapsed)
			if strings.Contains(rendered, "✦") || strings.Contains(rendered, "thinking") {
				t.Fatalf("cancelled message renders a live-thinking frame:\n%s", rendered)
			}
		}
	}
}
