package types

import "time"

// StreamEvent represents an event yielded during agent streaming.
type StreamEvent interface {
	isStreamEvent()
}

// StreamRequestStart signals the beginning of an API request.
type StreamRequestStart struct{}

func (StreamRequestStart) isStreamEvent() {}

// StreamMessage yields a partial or complete message during streaming.
type StreamMessage struct {
	Message Message
}

func (StreamMessage) isStreamEvent() {}

// TombstoneMessage removes an earlier message from the transcript.
type TombstoneMessage struct {
	TargetUUID string
}

func (TombstoneMessage) isStreamEvent() {}

// ProgressMessage carries tool execution progress for the UI.
type ProgressMessage struct {
	ToolUseID string    `json:"tool_use_id"`
	Type      string    `json:"type"`
	Data      any       `json:"data"`
	Timestamp time.Time `json:"timestamp"`
}

func (ProgressMessage) isStreamEvent() {}

// StreamError yields an error that occurred during streaming.
type StreamError struct {
	Error error
}

func (StreamError) isStreamEvent() {}

// LLMEvent is a single event from the LLM stream.
type LLMEvent interface {
	isLLMEvent()
}

// LLMTextDelta carries a fragment of assistant text.
type LLMTextDelta struct {
	Delta string
}

func (LLMTextDelta) isLLMEvent() {}

// LLMToolUseDelta carries a fragment of tool use input JSON.
type LLMToolUseDelta struct {
	ID    string
	Name  string
	Delta string // partial JSON
}

func (LLMToolUseDelta) isLLMEvent() {}

// LLMMessageStart signals the start of a message.
type LLMMessageStart struct {
	ID string
}

func (LLMMessageStart) isLLMEvent() {}

// LLMMessageStop signals the end of a message with metadata.
type LLMMessageStop struct {
	StopReason string
	Model      string
	Usage      TokenUsage
}

func (LLMMessageStop) isLLMEvent() {}

// LLMError carries a provider error.
type LLMError struct {
	Error error
}

func (LLMError) isLLMEvent() {}
