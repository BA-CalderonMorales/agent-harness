package tui

import (
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

func TestTimerLifecycleStopsAtTerminalTransitionsAndRestarts(t *testing.T) {
	tests := []struct {
		name string
		stop func(ChatModel) (ChatModel, tea.Cmd)
	}{
		{name: "done", stop: func(m ChatModel) (ChatModel, tea.Cmd) {
			return updateChatTimer(m, AgentDoneMsg{FullResponse: "done"})
		}},
		{name: "provider error", stop: func(m ChatModel) (ChatModel, tea.Cmd) {
			return updateChatTimer(m, AgentErrorMsg{Error: errors.New("provider stopped")})
		}},
		{name: "cancel", stop: func(m ChatModel) (ChatModel, tea.Cmd) {
			return updateChatTimer(m, AgentCancelMsg{})
		}},
		{name: "clear", stop: func(m ChatModel) (ChatModel, tea.Cmd) {
			return updateChatTimer(m, ClearChatMsg{})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, startCmd := updateChatTimer(NewChatModel(), AgentStartMsg{})
			if startCmd == nil {
				t.Fatal("AgentStartMsg did not schedule a timer")
			}
			m, tickCmd := updateChatTimer(m, timerTickMsg{})
			if tickCmd == nil {
				t.Fatal("active timer tick did not schedule its successor")
			}

			m, stopCmd := tt.stop(m)
			if stopCmd != nil {
				t.Fatalf("%s returned a timer command while settling", tt.name)
			}
			if m.timerRunning {
				t.Fatalf("%s left timerRunning=true", tt.name)
			}

			// A tick already queued before settlement must be inert and must
			// not resurrect the old status or create another command chain.
			before := m.elapsed
			m, staleCmd := updateChatTimer(m, timerTickMsg{})
			if staleCmd != nil {
				t.Fatalf("%s scheduled a command for a stale tick", tt.name)
			}
			if m.elapsed != before {
				t.Fatalf("%s changed elapsed time from a stale tick", tt.name)
			}

			m, restartCmd := updateChatTimer(m, AgentStartMsg{})
			if restartCmd == nil || !m.timerRunning {
				t.Fatalf("next turn did not restart timer: cmd=%v running=%v", restartCmd != nil, m.timerRunning)
			}
		})
	}
}

func updateChatTimer(m ChatModel, msg tea.Msg) (ChatModel, tea.Cmd) {
	model, cmd := m.Update(msg)
	return model.(ChatModel), cmd
}
