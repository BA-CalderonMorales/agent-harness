package types

import (
	"encoding/json"
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
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error,omitempty"`
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

// UnmarshalJSON restores concrete content block types from the persisted
// message format. ContentBlock is an interface, so encoding/json cannot decode
// it without this type dispatch.
func (m *Message) UnmarshalJSON(data []byte) error {
	type messageAlias Message
	var raw struct {
		messageAlias
		Content []json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	*m = Message(raw.messageAlias)
	m.Content = make([]ContentBlock, 0, len(raw.Content))
	for _, blockData := range raw.Content {
		block, err := unmarshalContentBlock(blockData)
		if err != nil {
			return err
		}
		m.Content = append(m.Content, block)
	}

	return nil
}

func unmarshalContentBlock(data []byte) (ContentBlock, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}

	switch {
	case fields["thinking"] != nil:
		var block ThinkingBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return nil, err
		}
		return block, nil
	case fields["tool_use_id"] != nil:
		var block ToolResultBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return nil, err
		}
		return block, nil
	case fields["id"] != nil || fields["name"] != nil || fields["input"] != nil:
		var block ToolUseBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return nil, err
		}
		return block, nil
	default:
		var block TextBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return nil, err
		}
		return block, nil
	}
}
