package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TestStreamingMessageSurvivesMidTurnPrepend guards the index-drift bug:
// a mid-turn PrependSystemNote (provider probe, auto-save notice) inserts
// at position 0 and used to shift currentStreamingAssistantIdx, so the
// streamed reply was written into the wrong message - the assistant
// bubble stayed empty while a system note absorbed the content.
func TestStreamingMessageSurvivesMidTurnPrepend(t *testing.T) {
	mm := NewChatModel()
	mm.width = 120
	mm.height = 40
	update := func(msg tea.Msg) {
		model, _ := mm.Update(msg)
		mm = model.(ChatModel)
	}

	update(AgentStartMsg{Timestamp: time.Now()})
	mm.startTime = time.Now().Add(-2 * time.Second)
	update(timerTickMsg{}) // materialize the placeholder

	update(AgentChunkMsg{Text: "Hello"})
	update(AgentChunkMsg{Text: " world"})

	// Mid-turn prepend: the note must NOT steal the stream.
	mm.PrependSystemNote("Provider ready: 1 models available")

	update(AgentChunkMsg{Text: "!"})
	update(AgentDoneMsg{FullResponse: ""})

	var assistant, note string
	for _, msg := range mm.messages {
		switch msg.Role {
		case "assistant":
			assistant = strings.TrimSpace(msg.Content)
		case "system":
			note = strings.TrimSpace(msg.Content)
		}
	}
	if assistant != "Hello world!" {
		t.Fatalf("assistant content = %q, want %q (the stream was stolen by a mid-turn prepend)", assistant, "Hello world!")
	}
	if note != "Provider ready: 1 models available" {
		t.Fatalf("system note = %q, want the probe notice intact", note)
	}
}

// TestStreamingMessageSurvivesMidTurnPrependMultiple: repeated prepends
// (probe + auto-save in one turn) must not corrupt the stream either.
func TestStreamingMessageSurvivesMidTurnPrependMultiple(t *testing.T) {
	mm := NewChatModel()
	mm.width = 120
	mm.height = 40
	update := func(msg tea.Msg) {
		model, _ := mm.Update(msg)
		mm = model.(ChatModel)
	}

	update(AgentStartMsg{Timestamp: time.Now()})
	mm.startTime = time.Now().Add(-2 * time.Second)
	update(timerTickMsg{})
	update(AgentChunkMsg{Text: "A"})
	mm.PrependSystemNote("Provider ready: 2 models available")
	update(AgentChunkMsg{Text: "B"})
	mm.PrependSystemNote("Auto-saved to /tmp/session.json")
	update(AgentChunkMsg{Text: "C"})
	update(AgentDoneMsg{FullResponse: ""})

	var assistant string
	for _, msg := range mm.messages {
		if msg.Role == "assistant" {
			assistant = strings.TrimSpace(msg.Content)
		}
	}
	if assistant != "ABC" {
		t.Fatalf("assistant content = %q, want %q", assistant, "ABC")
	}
}
