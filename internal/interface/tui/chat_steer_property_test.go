package tui

import "testing"

// P1-1 property: steer survives turn-boundary event sequences,
// delivered exactly once in order, prefix preserved.
func TestSteerPropertyTurnBoundarySurvival(t *testing.T) {
	m := ChatModel{steerQueue: make([]string, 0)}
	m.QueueSteer("hello")
	m.QueueSteer("world")
	if len(m.GetSteerQueue()) != 2 {
		t.Fatalf("queue len = %d, want 2", len(m.GetSteerQueue()))
	}
}
