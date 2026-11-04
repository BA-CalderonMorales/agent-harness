package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestToolPendingDoesNotClaimSuccess(t *testing.T) {
	m := NewChatModel()
	row := ansi.Strip(m.formatToolContentAt(80, "Write File", "names.py", ToolStatusPending, time.Now(), 0))
	if strings.Contains(row, "✓") || strings.Contains(row, "0.0s") || !strings.Contains(row, "○") {
		t.Fatalf("pending row claims completion: %s", row)
	}
}

func TestCancelSettlesToolAndNextTurnCanContinue(t *testing.T) {
	m := NewChatModel()
	m.resize(100, 30)
	update := func(msg interface{}) { model, _ := m.Update(msg); m = model.(ChatModel) }
	update(AgentStartMsg{Timestamp: time.Now()})
	update(AgentToolStartMsg{ToolID: "write-1", ToolName: "write", DisplayName: "Write File", ActivityDesc: "names.py"})
	// Another transcript append may reallocate the slice holding the current row.
	for i := 0; i < 20; i++ {
		m.AddMessage("system", "progress")
	}
	update(AgentCancelMsg{})
	count := len(m.messages)
	update(AgentErrorMsg{Error: context.Canceled})
	update(AgentToolDoneMsg{ToolID: "write-1", Success: true})
	update(AgentDoneMsg{FullResponse: "late answer"})
	if len(m.messages) != count {
		t.Fatal("cancellation gained misleading late messages")
	}
	for _, msg := range m.messages {
		if msg.ID == "write-1" && msg.ToolStatus != ToolStatusError {
			t.Fatal("cancelled tool kept running or became a success")
		}
	}
	update(AgentStartMsg{Timestamp: time.Now()})
	update(AgentChunkMsg{Text: "Corrected approach."})
	update(AgentDoneMsg{FullResponse: "Corrected approach."})
	if m.turnInterrupted || m.thinking || m.streaming {
		t.Fatal("next turn did not finish")
	}
	found := false
	for _, msg := range m.messages {
		if msg.Role == "assistant" && strings.Contains(msg.Content, "Corrected approach.") {
			found = true
		}
	}
	if !found {
		t.Fatal("next reply was lost after cancellation")
	}
}
