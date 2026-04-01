package types

import (
	"time"
)

// MessageRole identifies the sender of a message in the conversation.
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
)

// ContentBlock represents a single block of content in a message.
type ContentBlock interface {
	isContentBlock()
}

// TextBlock is plain text content.
type TextBlock struct {
	Text string `json:"text"`
}

func (TextBlock) isContentBlock() {}

// ToolUseBlock represents a request from the model to use a tool.
type ToolUseBlock struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

func (ToolUseBlock) isContentBlock() {}

// ToolResultBlock represents the result of a tool execution.
type ToolResultBlock struct {
	ToolUseID string      `json:"tool_use_id"`
	Content   string      `json:"content"`
	IsError   bool        `json:"is_error,omitempty"`
}

func (ToolResultBlock) isContentBlock() {}

// ThinkingBlock represents model reasoning (when enabled).
type ThinkingBlock struct {
	Thinking  string `json:"thinking"`
	Signature string `json:"signature,omitempty"`
}

func (ThinkingBlock) isContentBlock() {}

// Message is a single turn in the conversation.
type Message struct {
	UUID      string         `json:"uuid"`
	Role      MessageRole    `json:"role"`
	Content   []ContentBlock `json:"content"`
	Timestamp time.Time      `json:"timestamp"`

	// Metadata for internal tracking
	APIError   string `json:"api_error,omitempty"`
	StopReason string `json:"stop_reason,omitempty"`
	Model      string `json:"model,omitempty"`
}

// QuerySource identifies where a query originated.
type QuerySource string

const (
	SourceReplMainThread QuerySource = "repl_main_thread"
	SourceAgent          QuerySource = "agent"
	SourceCompact        QuerySource = "compact"
	SourceSessionMemory  QuerySource = "session_memory"
)

// TokenUsage tracks API token consumption.
type TokenUsage struct {
	InputTokens              int
	OutputTokens             int
	CacheReadInputTokens     int
	CacheCreationInputTokens int
}
