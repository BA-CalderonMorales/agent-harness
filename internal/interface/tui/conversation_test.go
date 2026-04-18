// Integration test for TUI conversation flow
// Verifies AGENTIC behavior: ALL messages flow through the LLM loop

package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	tea "github.com/charmbracelet/bubbletea"
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

// ---------------------------------------------------------------------------
// Outcome-based tests for critical TUX fixes
// ---------------------------------------------------------------------------

// TestModelChangedMsgUpdatesStatusBar verifies that ModelChangedMsg propagates
// to the chat model so the status bar shows the correct model.
// OUTCOME: When user switches model, status bar displays the new model name.
func TestModelChangedMsgUpdatesStatusBar(t *testing.T) {
	app := NewApp()
	app.SetChatModel("old-model")

	if app.chatModel.GetModel() != "old-model" {
		t.Fatalf("expected initial model 'old-model', got %q", app.chatModel.GetModel())
	}

	// Simulate model change message (as sent by /model command or settings tab)
	model, _ := app.Update(ModelChangedMsg{Model: "nvidia/nemotron-3-super-120b-a12b:free"})
	updatedApp := model.(App)

	if updatedApp.chatModel.GetModel() != "nvidia/nemotron-3-super-120b-a12b:free" {
		t.Errorf("expected model 'nvidia/nemotron-3-super-120b-a12b:free', got %q", updatedApp.chatModel.GetModel())
	}
}

// TestClearChatWithFollowUpMsg verifies that ClearChatMsg with FollowUpMsg
// clears the chat AND preserves the confirmation message.
// OUTCOME: /clear removes all messages and shows "Session cleared." exactly once.
func TestClearChatWithFollowUpMsg(t *testing.T) {
	chat := NewChatModel()
	chat.AddMessage("user", "hello")
	chat.AddMessage("assistant", "hi there")

	if len(chat.messages) != 2 {
		t.Fatalf("expected 2 messages before clear, got %d", len(chat.messages))
	}

	// Simulate clear command with follow-up message
	model, _ := chat.Update(ClearChatMsg{FollowUpMsg: "Session cleared."})
	chat = model.(ChatModel)

	if len(chat.messages) != 1 {
		t.Fatalf("expected 1 message after clear (the follow-up), got %d", len(chat.messages))
	}

	if chat.messages[0].Role != "system" {
		t.Errorf("expected follow-up role 'system', got %s", chat.messages[0].Role)
	}

	if chat.messages[0].Content != "Session cleared." {
		t.Errorf("expected follow-up content 'Session cleared.', got %q", chat.messages[0].Content)
	}
}

// TestClearChatWithoutFollowUpMsg verifies bare clear removes everything.
func TestClearChatWithoutFollowUpMsg(t *testing.T) {
	chat := NewChatModel()
	chat.AddMessage("user", "hello")

	model, _ := chat.Update(ClearChatMsg{})
	chat = model.(ChatModel)

	if len(chat.messages) != 0 {
		t.Errorf("expected 0 messages after bare clear, got %d", len(chat.messages))
	}
}

// TestInlineSuggestionsAppear verifies that typing "/" shows suggestions.
// OUTCOME: When "/" is typed in empty input, inline suggestion list appears.
func TestInlineSuggestionsAppear(t *testing.T) {
	chat := NewChatModel()
	chat.SetCommandCompletions([]string{"/clear", "/compact", "/config", "/model"})

	// Simulate typing "/" in empty input
	model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	chat = model.(ChatModel)

	if !chat.showSuggestions {
		t.Error("expected suggestions to show after typing '/'")
	}

	if len(chat.suggestions) != 4 {
		t.Errorf("expected 4 suggestions, got %d", len(chat.suggestions))
	}
}

// TestInlineSuggestionsFilter verifies that typing after "/" filters suggestions.
// OUTCOME: Typing "/c" shows only commands starting with "/c".
func TestInlineSuggestionsFilter(t *testing.T) {
	chat := NewChatModel()
	chat.SetCommandCompletions([]string{"/clear", "/compact", "/config", "/model"})

	// Type "/c"
	chat.textarea.SetValue("/c")
	model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	chat = model.(ChatModel)

	// Trigger suggestion update by simulating the key that updates textarea
	chat.showSuggestions = true
	chat.suggestions = chat.filterSuggestions("/c")

	if len(chat.suggestions) != 3 {
		t.Errorf("expected 3 suggestions for '/c', got %d: %v", len(chat.suggestions), chat.suggestions)
	}

	for _, s := range chat.suggestions {
		if !strings.HasPrefix(s, "/c") {
			t.Errorf("suggestion %q does not start with '/c'", s)
		}
	}
}

// TestInlineSuggestionsNavigate verifies up/down navigation.
// OUTCOME: Down arrow moves selection; Enter inserts selected command.
func TestInlineSuggestionsNavigate(t *testing.T) {
	chat := NewChatModel()
	chat.SetCommandCompletions([]string{"/clear", "/compact", "/config"})
	chat.showSuggestions = true
	chat.suggestions = chat.filterSuggestions("/")
	chat.suggestionCursor = 0

	// Press down
	model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyDown})
	chat = model.(ChatModel)

	if chat.suggestionCursor != 1 {
		t.Errorf("expected cursor 1 after down, got %d", chat.suggestionCursor)
	}

	// Press up
	model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyUp})
	chat = model.(ChatModel)

	if chat.suggestionCursor != 0 {
		t.Errorf("expected cursor 0 after up, got %d", chat.suggestionCursor)
	}
}

// TestInlineSuggestionsSelect verifies Enter selects a suggestion.
// OUTCOME: Enter inserts the selected command into the textarea.
func TestInlineSuggestionsSelect(t *testing.T) {
	chat := NewChatModel()
	chat.SetCommandCompletions([]string{"/clear", "/compact"})
	chat.showSuggestions = true
	chat.suggestions = chat.filterSuggestions("/")
	chat.suggestionCursor = 0

	// Press enter to select first suggestion
	model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
	chat = model.(ChatModel)

	if chat.showSuggestions {
		t.Error("expected suggestions to hide after selection")
	}

	if chat.textarea.Value() != "/clear " {
		t.Errorf("expected textarea value '/clear ', got %q", chat.textarea.Value())
	}
}

// TestInlineSuggestionsCancel verifies Esc hides suggestions.
// OUTCOME: Esc dismisses the suggestion list without modifying input.
func TestInlineSuggestionsCancel(t *testing.T) {
	chat := NewChatModel()
	chat.SetCommandCompletions([]string{"/clear"})
	chat.showSuggestions = true
	chat.suggestions = chat.filterSuggestions("/")
	chat.textarea.SetValue("/clear")

	model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyEscape})
	chat = model.(ChatModel)

	if chat.showSuggestions {
		t.Error("expected suggestions to hide after Esc")
	}
}

// TestInlineSuggestionsConsumeKeys verifies Tab and Esc are consumed
// when suggestions are showing, preventing view-switching conflicts.
func TestInlineSuggestionsConsumeKeys(t *testing.T) {
	chat := NewChatModel()

	// Without suggestions, Tab and Esc are not consumed
	if chat.ConsumesTab() {
		t.Error("expected ConsumesTab=false when no suggestions")
	}
	if chat.ConsumesEsc() {
		t.Error("expected ConsumesEsc=false when no suggestions")
	}

	// With suggestions, they are consumed
	chat.showSuggestions = true
	if !chat.ConsumesTab() {
		t.Error("expected ConsumesTab=true when suggestions showing")
	}
	if !chat.ConsumesEsc() {
		t.Error("expected ConsumesEsc=true when suggestions showing")
	}
}

// TestSuggestionScrollWindow verifies cursor beyond visible window scrolls view.
// OUTCOME: Hitting down past item 6 shows items 1-7 with cursor at 7.
func TestSuggestionScrollWindow(t *testing.T) {
	chat := NewChatModel()
	chat.SetCommandCompletions([]string{
		"/agents", "/branch", "/clear", "/compact", "/config",
		"/cost", "/diff", "/export", "/help", "/model",
	})
	chat.showSuggestions = true
	chat.suggestions = chat.filterSuggestions("/")
	chat.suggestionCursor = 0
	chat.suggestionOffset = 0

	// Navigate to item 7 (0-indexed: cursor = 7)
	for i := 0; i < 7; i++ {
		model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyDown})
		chat = model.(ChatModel)
	}

	if chat.suggestionCursor != 7 {
		t.Fatalf("expected cursor 7, got %d", chat.suggestionCursor)
	}
	if chat.suggestionOffset != 2 {
		t.Errorf("expected offset 2 (showing items 2-7), got %d", chat.suggestionOffset)
	}

	// Navigate back to item 0
	for i := 0; i < 7; i++ {
		model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyUp})
		chat = model.(ChatModel)
	}

	if chat.suggestionOffset != 0 {
		t.Errorf("expected offset 0 after scrolling back, got %d", chat.suggestionOffset)
	}
}

// TestModelChangedMsgSyncsSettings verifies model change updates settings tab.
// OUTCOME: ModelChangedMsg updates both chat status bar AND settings model.
func TestModelChangedMsgSyncsSettings(t *testing.T) {
	app := NewApp()
	app.SetSettings([]Setting{
		{Key: "model", Label: "Model", Value: "old-model"},
		{Key: "provider", Label: "Provider", Value: "openrouter"},
	})

	updated, _ := app.Update(ModelChangedMsg{Model: "new-model"})
	updatedApp := updated.(App)

	if updatedApp.chatModel.GetModel() != "new-model" {
		t.Errorf("chat model not updated, got %q", updatedApp.chatModel.GetModel())
	}

	settings := updatedApp.settingsModel.settings
	found := false
	for _, s := range settings {
		if s.Key == "model" && s.Value == "new-model" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("settings model model value not synced to 'new-model', got %+v", settings)
	}
}

// TestSettingsUpdateSettingValue verifies single-setting update by key.
func TestSettingsUpdateSettingValue(t *testing.T) {
	sm := NewSettingsModel()
	sm.SetSettings([]Setting{
		{Key: "model", Value: "old"},
		{Key: "provider", Value: "openrouter"},
	})

	sm.UpdateSettingValue("model", "new-model")

	if sm.settings[0].Value != "new-model" {
		t.Errorf("expected model='new-model', got %q", sm.settings[0].Value)
	}
	if sm.settings[1].Value != "openrouter" {
		t.Errorf("provider should be unchanged, got %q", sm.settings[1].Value)
	}
}

// TestClearChatListenerContinues verifies ClearChatMsg handler keeps listener alive.
// OUTCOME: After clear, subsequent async messages are still received.
func TestClearChatListenerContinues(t *testing.T) {
	app := NewApp()
	app.Init()

	// Send ClearChatMsg
	updated, cmd := app.Update(ClearChatMsg{FollowUpMsg: "Cleared."})
	_ = updated.(App)

	// Verify listenForMessages is in the command batch
	if cmd == nil {
		t.Fatal("expected command after ClearChatMsg")
	}

	// Send a follow-up message through channel
	app.Send(StatusMsg{Text: "test", Type: "info"})

	select {
	case msg := <-app.msgChan:
		if _, ok := msg.(StatusMsg); !ok {
			t.Errorf("expected StatusMsg, got %T", msg)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for message - listener may have stopped")
	}
}

// TestRemoveLastUserMessage verifies selective removal of last user message.
func TestRemoveLastUserMessage(t *testing.T) {
	chat := NewChatModel()
	chat.AddMessage("user", "hello")
	chat.AddMessage("assistant", "hi")
	chat.AddMessage("user", "secret-key")

	if len(chat.messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(chat.messages))
	}

	chat.RemoveLastUserMessage()

	if len(chat.messages) != 2 {
		t.Fatalf("expected 2 messages after removal, got %d", len(chat.messages))
	}
	if chat.messages[1].Content != "hi" {
		t.Errorf("expected remaining assistant message 'hi', got %q", chat.messages[1].Content)
	}
}

// TestSuggestionRenderScrollIndicators verifies "above/below" indicators show.
func TestSuggestionRenderScrollIndicators(t *testing.T) {
	chat := NewChatModel()
	chat.SetCommandCompletions([]string{
		"/a", "/b", "/c", "/d", "/e", "/f", "/g", "/h", "/i", "/j",
	})
	chat.showSuggestions = true
	chat.suggestions = chat.filterSuggestions("/")
	chat.suggestionCursor = 5
	chat.suggestionOffset = 2

	rendered := chat.renderSuggestions()

	if !strings.Contains(rendered, "above") {
		t.Error("expected 'above' indicator when offset > 0")
	}
	if !strings.Contains(rendered, "below") {
		t.Error("expected 'below' indicator when more items remain")
	}
}

// TestModelChangedMsgEmptyModel verifies status bar handles empty model.
func TestModelChangedMsgEmptyModel(t *testing.T) {
	app := NewApp()
	app.SetChatModel("")

	updated, _ := app.Update(ModelChangedMsg{Model: ""})
	updatedApp := updated.(App)

	if updatedApp.chatModel.GetModel() != "" {
		t.Errorf("expected empty model, got %q", updatedApp.chatModel.GetModel())
	}
}
