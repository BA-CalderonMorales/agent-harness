package tui

import (
	"testing"
	"time"
)

// Ordering regression: tool calls that arrive after the streaming
// assistant bubble materialized used to append BELOW the answer, so a
// multi-step turn rendered its final response and then trailing tool
// lines under it. Within a turn the work precedes the answer: the
// assistant bubble stays pinned last while tool messages insert before
// it.
func TestToolCallsStayAboveTheStreamingAnswer(t *testing.T) {
	chat := NewChatModel()
	chat.width = 120
	chat.height = 24
	chat.resize(120, 24)
	chat.focused = true

	// Turn starts; the placeholder defers the assistant bubble.
	model, _ := chat.Update(AgentStartMsg{})
	chat = model.(ChatModel)
	// Age the turn past PlaceholderDelay without sleeping.
	chat.startTime = time.Now().Add(-2 * time.Second)

	model, _ = chat.Update(AgentToolStartMsg{ToolID: "t1", ToolName: "bash", DisplayName: "bash"})
	chat = model.(ChatModel)

	// The placeholder delay elapses: the assistant bubble materializes.
	model, _ = chat.Update(timerTickMsg{time: time.Now()})
	chat = model.(ChatModel)
	model, _ = chat.Update(AgentChunkMsg{Text: "partial answer"})
	chat = model.(ChatModel)

	// A second tool starts AFTER the answer bubble exists.
	model, _ = chat.Update(AgentToolStartMsg{ToolID: "t2", ToolName: "read", DisplayName: "read"})
	chat = model.(ChatModel)

	if n := len(chat.messages); n != 3 {
		t.Fatalf("messages = %d, want 3 (tool t1, tool t2, streaming answer)", n)
	}
	last := chat.messages[len(chat.messages)-1]
	if last.Role != "assistant" {
		t.Fatalf("last message role = %q, want assistant: tool calls must stay above the streaming answer", last.Role)
	}
	if chat.messages[0].ID != "t1" || chat.messages[1].ID != "t2" {
		t.Fatalf("tool order = [%s, %s], want t1 before t2", chat.messages[0].ID, chat.messages[1].ID)
	}

	// The turn ends: the answer finalizes in place, still last. The
	// stream buffer outranks FullResponse once chunks streamed.
	model, _ = chat.Update(AgentDoneMsg{FullResponse: "final answer"})
	chat = model.(ChatModel)
	last = chat.messages[len(chat.messages)-1]
	if last.Role != "assistant" || last.Content != "partial answer" {
		t.Fatalf("final message = (%s, %q), want the streamed assistant answer last", last.Role, last.Content)
	}
}
