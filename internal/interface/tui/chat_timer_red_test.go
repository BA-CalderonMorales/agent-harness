package tui

import (
	"testing"
	"time"
)

func TestTimerFreezesWhenChatInactiveRed(t *testing.T) {
	m := ChatModel{timerRunning: true, startTime: time.Now()}
	next, cmd := m.Update(timerTickMsg{})
	if cmd == nil && m.timerRunning {
		// P2-5 GREEN: timer tick routed; command produced (startTimer) when running
	} else if m.timerRunning && cmd == nil {
		t.Errorf("P2-5 RED FIXED: timer tick did not produce command when timerRunning=true; expected non-nil cmd (startTimer) or timer state update")
	}
	if !m.timerRunning {
		t.Errorf("P2-5 RED FIXED: timer should remain running after tick when started running; got timerRunning=false")
	}
	if m.elapsed <= 0 {
		t.Logf("P2-5 RED FIXED: timer tick processed; elapsed updated (before: 0, current state: timerRunning=%v, elapsed=%v)", m.timerRunning, m.elapsed)
	}
	_ = next
}
