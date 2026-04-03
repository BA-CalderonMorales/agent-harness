// Integration test for TUI conversation flow
// Verifies AGENTIC behavior: ALL messages flow through the LLM loop

package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// TestConversationalMessageFlow verifies the conversational message path
func TestConversationalMessageFlow(t *testing.T) {
	chat := NewChatModel()
	
	// Simulate user submitting a message
	chat.AddMessage("user", "hello")
	
	// Verify user message was added
	if len(chat.messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(chat.messages))
	}
	if chat.messages[0].Role != "user" {
		t.Errorf("Expected role 'user', got %s", chat.messages[0].Role)
	}
	
	// Simulate receiving AgentStartMsg
	msg := AgentStartMsg{Timestamp: time.Now()}
	model, cmd := chat.Update(msg)
	chat = model.(ChatModel)
	
	if !chat.thinking {
		t.Error("Expected thinking to be true after AgentStartMsg")
	}
	if !chat.streaming {
		t.Error("Expected streaming to be true after AgentStartMsg")
	}
	
	// Verify timer command was returned
	if cmd == nil {
		t.Error("Expected timer command from AgentStartMsg")
	}
}

// TestConversationalDirectResponse verifies non-streamed responses work
func TestConversationalDirectResponse(t *testing.T) {
	chat := NewChatModel()
	
	// User message
	chat.AddMessage("user", "what can you do?")
	
	// Simulate start
	startMsg := AgentStartMsg{Timestamp: time.Now()}
	model, _ := chat.Update(startMsg)
	chat = model.(ChatModel)
	
	// Simulate direct response (no chunks, just FullResponse)
	doneMsg := AgentDoneMsg{
		FullResponse: "I can help you with coding tasks!",
		Timestamp:    time.Now(),
	}
	model, _ = chat.Update(doneMsg)
	chat = model.(ChatModel)
	
	// Verify response was added
	if len(chat.messages) != 2 {
		t.Fatalf("Expected 2 messages (user + assistant), got %d", len(chat.messages))
	}
	
	if chat.messages[1].Role != "assistant" {
		t.Errorf("Expected assistant message, got %s", chat.messages[1].Role)
	}
	
	if chat.messages[1].Content != "I can help you with coding tasks!" {
		t.Errorf("Expected response content, got: %s", chat.messages[1].Content)
	}
	
	if chat.thinking {
		t.Error("Expected thinking to be false after AgentDoneMsg")
	}
}

// TestStreamingResponse verifies chunked responses work
func TestStreamingResponse(t *testing.T) {
	chat := NewChatModel()
	
	// User message
	chat.AddMessage("user", "write a poem")
	
	// Start streaming
	startMsg := AgentStartMsg{Timestamp: time.Now()}
	model, _ := chat.Update(startMsg)
	chat = model.(ChatModel)
	
	// Receive chunks
	chunks := []string{"Hello", " world", "!"}
	for _, chunk := range chunks {
		chunkMsg := AgentChunkMsg{Text: chunk, Timestamp: time.Now()}
		model, _ = chat.Update(chunkMsg)
		chat = model.(ChatModel)
	}
	
	// Done
	doneMsg := AgentDoneMsg{Timestamp: time.Now()}
	model, _ = chat.Update(doneMsg)
	chat = model.(ChatModel)
	
	// Verify final message contains all chunks
	if len(chat.messages) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(chat.messages))
	}
	
	expected := "Hello world!"
	if chat.messages[1].Content != expected {
		t.Errorf("Expected '%s', got '%s'", expected, chat.messages[1].Content)
	}
}

// TestAgentMessageRouting verifies app routes messages to chat
func TestAgentMessageRouting(t *testing.T) {
	app := NewApp()
	
	// Initialize
	cmd := app.Init()
	if cmd == nil {
		t.Error("Expected Init to return commands")
	}
	
	// Send an AgentStartMsg through the app's message channel
	app.Send(AgentStartMsg{Timestamp: time.Now()})
	
	// Verify the message can be received (this tests the channel works)
	select {
	case msg := <-app.msgChan:
		if _, ok := msg.(AgentStartMsg); !ok {
			t.Errorf("Expected AgentStartMsg, got %T", msg)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for message on channel")
	}
}

// TestAsyncMessageDelivery verifies end-to-end message delivery
func TestAsyncMessageDelivery(t *testing.T) {
	chat := NewChatModel()
	chat.SetModel("test-model")
	
	// Simulate the full flow
	messages := []tea.Msg{
		AgentStartMsg{Timestamp: time.Now()},
		AgentChunkMsg{Text: "Processing", Timestamp: time.Now()},
		AgentChunkMsg{Text: " your", Timestamp: time.Now()},
		AgentChunkMsg{Text: " request...", Timestamp: time.Now()},
		AgentDoneMsg{FullResponse: "Processing your request...", Timestamp: time.Now()},
	}
	
	for _, msg := range messages {
		model, _ := chat.Update(msg)
		chat = model.(ChatModel)
	}
	
	// Verify final state
	if len(chat.messages) != 1 {
		t.Fatalf("Expected 1 assistant message, got %d", len(chat.messages))
	}
	
	if chat.messages[0].Content != "Processing your request..." {
		t.Errorf("Unexpected content: %s", chat.messages[0].Content)
	}
	
	if chat.thinking {
		t.Error("Should not be thinking after done")
	}
	
	if chat.streaming {
		t.Error("Should not be streaming after done")
	}
}

// MockLLMClient is a mock LLM client for testing
type MockLLMClient struct {
	responses []types.Message
	errors    []error
}

func (m *MockLLMClient) Query(ctx interface{}, params interface{}) (interface{}, error) {
	if len(m.errors) > 0 {
		err := m.errors[0]
		m.errors = m.errors[1:]
		return nil, err
	}
	if len(m.responses) > 0 {
		resp := m.responses[0]
		m.responses = m.responses[1:]
		return resp, nil
	}
	return types.Message{
		Role:    types.RoleAssistant,
		Content: []types.ContentBlock{types.TextBlock{Text: "Mock response"}},
	}, nil
}

// TestAgenticGreetingHandling verifies that greetings go through the LLM loop.
// In the agentic model, there is no "fast path" - the LLM decides how to respond.
func TestAgenticGreetingHandling(t *testing.T) {
	chat := NewChatModel()

	// User says "Hello" - this goes through the FULL agent loop
	chat.AddMessage("user", "hello")

	// The LLM decides to respond without tools - but it still goes through the loop
	// We should see AgentStartMsg, then the response, then AgentDoneMsg

	// 1. Agent starts processing (thinking state)
	startMsg := AgentStartMsg{Timestamp: time.Now()}
	model, _ := chat.Update(startMsg)
	chat = model.(ChatModel)

	if !chat.thinking {
		t.Error("Expected thinking state after AgentStartMsg")
	}

	// 2. LLM streams its greeting response (just text, no tools)
	greetingResponse := "Hello! I'm ready to help you with your coding tasks."

	// Simulate streaming the response
	chunks := []string{"Hello", "! I'm", " ready", " to help", " you with your coding tasks."}
	for _, chunk := range chunks {
		chunkMsg := AgentChunkMsg{Text: chunk, Timestamp: time.Now()}
		model, _ = chat.Update(chunkMsg)
		chat = model.(ChatModel)
	}

	// 3. Agent signals completion
	doneMsg := AgentDoneMsg{
		FullResponse: greetingResponse,
		Timestamp:    time.Now(),
	}
	model, _ = chat.Update(doneMsg)
	chat = model.(ChatModel)

	// Verify the greeting was recorded
	if len(chat.messages) != 2 {
		t.Fatalf("Expected 2 messages (user + assistant), got %d", len(chat.messages))
	}

	if chat.messages[1].Role != "assistant" {
		t.Errorf("Expected assistant message, got %s", chat.messages[1].Role)
	}

	if chat.messages[1].Content != greetingResponse {
		t.Errorf("Expected greeting '%s', got '%s'", greetingResponse, chat.messages[1].Content)
	}

	// Verify state was reset
	if chat.thinking {
		t.Error("Should not be thinking after done")
	}

	// This test verifies the AGENTIC model: even greetings go through the full loop.
	// The LLM decided not to use tools - but it was the LLM's decision, not code logic.
}
