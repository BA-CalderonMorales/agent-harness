package tui

import "testing"

func TestRapidStreamingMessagesHaveDistinctIDs(t *testing.T) {
	model := NewChatModel()
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		model.currentStreamingAssistantID = ""
		model.updateOrCreateStreamingMessage("")
		id := model.currentStreamingAssistantID
		if seen[id] {
			t.Fatalf("two assistant messages share ID %q", id)
		}
		seen[id] = true
	}
}
