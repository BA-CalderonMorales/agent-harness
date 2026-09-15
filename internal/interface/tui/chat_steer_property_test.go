package tui

import (
	"testing"
	"time"
)

// P1-1 property: steer survives turn-boundary event sequences,
// delivered exactly once in order, prefix preserved.
func TestSteerPropertyTurnBoundarySurvival(t *testing.T) {
	m := ChatModel{steerQueue: make([]queuedSubmit, 0)}
	m.QueueSteer("hello")
	m.QueueSteer("world")
	if len(m.GetSteerQueue()) != 2 {
		t.Fatalf("queue len = %d, want 2", len(m.GetSteerQueue()))
	}
}

// TestSubmitWhileTurnInFlightQueues pins the mid-flight contract: a
// message submitted while the agent is already working must queue, not
// start a second concurrent turn (two interleaved streams corrupted the
// session). The bubble lands immediately; AgentDoneMsg then runs the
// message exactly once without repeating the user bubble.
func TestSubmitWhileTurnInFlightQueues(t *testing.T) {
	h := newSubmitDebounceHarness()
	h.model.textarea.SetValue("ask about the tests")

	// Mid-turn: AgentStartMsg sets the thinking/streaming flags.
	updated, _ := h.model.Update(AgentStartMsg{Timestamp: time.Now()})
	h.model = updated.(ChatModel)
	if !h.model.turnInFlight() {
		t.Fatal("AgentStartMsg did not mark the turn in flight")
	}

	chat, cmd := h.model.doSubmit()
	h.model = chat

	if len(h.subs) != 0 {
		t.Fatalf("mid-turn submit started a turn: %v", h.subs)
	}
	if got := h.model.GetSteerQueue(); len(got) != 1 || got[0] != "ask about the tests" {
		t.Fatalf("mid-turn submit not queued: %v", got)
	}
	// The user bubble is on screen now, and only once.
	if n := len(h.model.messages); n != 1 || h.model.messages[0].Content != "ask about the tests" {
		t.Fatalf("transcript = %+v", h.model.messages)
	}
	// The user is told their message was queued.
	if cmd == nil {
		t.Fatal("mid-turn submit returned no status command")
	}
	if _, ok := cmd().(StatusMsg); !ok {
		t.Fatalf("status cmd msg = %T, want StatusMsg", cmd())
	}

	// The turn finishes: the queued message submits exactly once and the
	// bubble is not repeated.
	updated, _ = h.model.Update(AgentDoneMsg{})
	h.model = updated.(ChatModel)
	if len(h.subs) != 1 || h.subs[0] != "ask about the tests" {
		t.Fatalf("auto-submit = %v, want exactly one submit", h.subs)
	}
	if n := len(h.model.messages); n != 1 {
		t.Fatalf("queued submit duplicated the user bubble: %d messages", n)
	}
	if len(h.model.GetSteerQueue()) != 0 {
		t.Fatalf("queue not drained: %v", h.model.GetSteerQueue())
	}
}
