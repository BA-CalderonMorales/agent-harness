// Message types for async communication between agent loop and TUI
// Following lumina-bot's pattern for streaming integration

package tui

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/approval"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// StreamStartMsg is sent when the agent starts processing a message
type StreamStartMsg struct {
	Prompt string
}

// StreamChunkMsg contains a text chunk from the LLM stream
type StreamChunkMsg struct {
	Text string
}

// StreamToolMsg contains a tool execution event
type StreamToolMsg struct {
	Name   string
	Input  map[string]any
	Result string
}

// StreamMessageMsg contains a complete message from the stream
type StreamMessageMsg struct {
	Message types.Message
}

// StreamErrorMsg contains an error from the stream
type StreamErrorMsg struct {
	Error string
}

// StreamDoneMsg is sent when the stream completes
type StreamDoneMsg struct {
	TurnCount int
}

// AgentResponseMsg is a complete response (for non-streaming fallback)
type AgentResponseMsg struct {
	Role    string
	Content string
}

// QuitMsg signals the TUI should exit
type QuitMsg struct{}

// openCommandPaletteMsg signals the command palette should open
type openCommandPaletteMsg struct{}

// openModelPickerMsg signals the model picker should open
type openModelPickerMsg struct{}

// ClearChatMsg signals the chat should be cleared
type ClearChatMsg struct{}

// ToolExecutingMsg is sent when a tool is about to execute (for visibility)
type ToolExecutingMsg struct {
	ToolName string
	Command  string
}

// ApprovalRequestMsg is sent when command approval is needed
type ApprovalRequestMsg struct {
	Request *approval.ApprovalRequest
}

// AgentCancelMsg is sent when the user cancels agent execution (ESC key)
type AgentCancelMsg struct{}
