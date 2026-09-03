// Message types for async communication between agent loop and TUI
// Following lumina-bot's pattern for streaming integration

package tui

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
	"github.com/BA-CalderonMorales/agent-harness/pkg/git"
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

// LoginCompletedMsg lands after the login wizard finishes: the app
// switches to chat in insert mode, ready to type - the first-run happy
// path never strands the user on the home screen.
type LoginCompletedMsg struct{}

// openModelPickerMsg signals the model picker should open
type openModelPickerMsg struct{}

// openProviderPickerMsg signals the provider-switch modal should open
type openProviderPickerMsg struct{}

// ClearChatMsg signals the chat should be cleared.
// If FollowUpMsg is set, it is added after clearing (atomically, avoiding races).
type ClearChatMsg struct {
	FollowUpMsg string
}

// ToolExecutingMsg is sent when a tool is about to execute (for visibility)
type ToolExecutingMsg struct {
	ToolID   string // Unique ID for message tracking/replacement
	ToolName string
	Command  string
}

// ApprovalRequestMsg is sent when command approval is needed
type ApprovalRequestMsg struct {
	Request *approval.ApprovalRequest
}

// AgentCancelMsg is sent when the user cancels agent execution (ESC key)
type AgentCancelMsg struct{}

// ProviderReadinessMsg reports the provider's readiness state.
type ProviderReadinessMsg struct {
	Readiness int // 0=checking, 1=ready, 2=warning, 3=unavailable, 4=misconfigured
	Message   string
	Model     string
	Endpoint  string
	// Gen is the probe generation the result belongs to; results from a
	// generation older than the current one are stale and discarded
	// (0 = no generation, always applied, e.g. boot notices).
	Gen int
}

type SessionActivatedMsg struct {
	SessionID      string
	Transcript     []types.Message
	Model          string
	Persona        string
	Sessions       []SessionInfo
	Notice         string
	NoticeType     string
	SwitchToChat   bool
	PermissionMode string
	EstTokens      int
}

type SessionsRefreshedMsg struct {
	Sessions   []SessionInfo
	Notice     string
	NoticeType string
}

// SwitchViewMsg requests switching the active tab view (0=Home, 1=Chat, 2=Sessions, 3=Settings)
type SwitchViewMsg struct {
	View viewID
}

// GitContextMsg delivers the collected git context after boot; the
// dashboard and welcome populate when it lands instead of blocking the
// TUI start.
type GitContextMsg struct {
	Context *git.Context
}
