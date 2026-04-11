package buckets

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// ConversationBucket handles conversation state and message management.
// It implements AgentBase for conversation operations.
type ConversationBucket struct {
	history []types.Message
}

// Conversation creates a new conversation bucket.
func Conversation() *ConversationBucket {
	return &ConversationBucket{
		history: make([]types.Message, 0),
	}
}

// Name returns the bucket identifier.
func (c *ConversationBucket) Name() string {
	return "conversation"
}

// CanHandle determines if this bucket handles the operation.
func (c *ConversationBucket) CanHandle(operation string, params map[string]any) bool {
	switch operation {
	case "add_message", "get_history", "clear_history", "get_context", "classify_input":
		return true
	}
	return false
}

// Capabilities describes what this bucket can do.
func (c *ConversationBucket) Capabilities() agent.AgentBucketCapabilities {
	return agent.AgentBucketCapabilities{
		IsConcurrencySafe: false, // Conversation is stateful
		IsStateful:        true,
		Operations:        []string{"add_message", "get_history", "clear_history", "get_context", "classify_input"},
		Category:          "conversation",
	}
}

// Execute runs the conversation operation.
func (c *ConversationBucket) Execute(ctx agent.AgentExecutionContext) agent.AgentResult {
	switch ctx.Operation {
	case "add_message":
		return c.handleAddMessage(ctx)
	case "get_history":
		return c.handleGetHistory(ctx)
	case "clear_history":
		return c.handleClearHistory(ctx)
	case "get_context":
		return c.handleGetContext(ctx)
	case "classify_input":
		return c.handleClassifyInput(ctx)
	default:
		return agent.AgentResult{
			Success: false,
			Error:   agent.NewAgentError("unknown_operation", "conversation doesn't handle: "+ctx.Operation),
		}
	}
}

// handleAddMessage adds a message to the conversation.
func (c *ConversationBucket) handleAddMessage(ctx agent.AgentExecutionContext) agent.AgentResult {
	msg, ok := ctx.Params["message"].(types.Message)
	if !ok {
		return agent.AgentResult{
			Success: false,
			Error:   agent.NewAgentError("invalid_input", "message required"),
		}
	}

	c.history = append(c.history, msg)

	return agent.AgentResult{
		Success: true,
		Data:    "message added",
	}
}

// handleGetHistory returns the conversation history.
func (c *ConversationBucket) handleGetHistory(ctx agent.AgentExecutionContext) agent.AgentResult {
	// Return copy to prevent external modification
	history := make([]types.Message, len(c.history))
	copy(history, c.history)

	return agent.AgentResult{
		Success:  true,
		Data:     history,
		Messages: history,
	}
}

// handleClearHistory clears the conversation history.
func (c *ConversationBucket) handleClearHistory(ctx agent.AgentExecutionContext) agent.AgentResult {
	c.history = make([]types.Message, 0)

	return agent.AgentResult{
		Success: true,
		Data:    "history cleared",
	}
}

// handleGetContext returns the current context for LLM calls.
func (c *ConversationBucket) handleGetContext(ctx agent.AgentExecutionContext) agent.AgentResult {
	// Combine bucket history with context messages
	history := make([]types.Message, len(c.history))
	copy(history, c.history)

	if len(ctx.Messages) > 0 {
		history = append(history, ctx.Messages...)
	}

	return agent.AgentResult{
		Success:  true,
		Data:     history,
		Messages: history,
	}
}

// handleClassifyInput classifies user input.
func (c *ConversationBucket) handleClassifyInput(ctx agent.AgentExecutionContext) agent.AgentResult {
	input, ok := ctx.Params["input"].(string)
	if !ok {
		return agent.AgentResult{
			Success: false,
			Error:   agent.NewAgentError("invalid_input", "input string required"),
		}
	}

	convType := agent.ClassifyInput(input)

	return agent.AgentResult{
		Success: true,
		Data:    convType,
	}
}

// GetHistory returns the conversation history.
func (c *ConversationBucket) GetHistory() []types.Message {
	history := make([]types.Message, len(c.history))
	copy(history, c.history)
	return history
}

// AddMessage adds a message to the history.
func (c *ConversationBucket) AddMessage(msg types.Message) {
	c.history = append(c.history, msg)
}

// Clear clears the conversation history.
func (c *ConversationBucket) Clear() {
	c.history = make([]types.Message, 0)
}

// Ensure ConversationBucket implements AgentBase
var _ agent.AgentBase = (*ConversationBucket)(nil)
