// Async agent integration for real-time streaming responses
// Enables non-blocking LLM queries with live token display

package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// getToolDisplayName returns a user-friendly display name for a tool
func getToolDisplayName(toolName string) string {
	switch toolName {
	case "bash", "BashTool":
		return "Shell"
	case "read", "ReadTool":
		return "Read File"
	case "write", "WriteTool":
		return "Write File"
	case "edit", "EditTool":
		return "Edit File"
	case "glob", "GlobTool":
		return "Find Files"
	case "grep", "GrepTool":
		return "Search"
	case "webfetch", "WebFetchTool":
		return "Fetch URL"
	case "websearch", "WebSearchTool":
		return "Web Search"
	case "agent", "AgentTool":
		return "Agent"
	case "ask", "AskTool":
		return "Ask"
	case "todo", "TodoTool":
		return "Todo"
	case "note", "NotebookTool":
		return "Notebook"
	case "plan", "PlanTool":
		return "Plan"
	case "settings", "SettingsTool":
		return "Settings"
	case "export", "ExportTool":
		return "Export"
	case "rewind", "RewindTool":
		return "Rewind"
	default:
		// Capitalize first letter as fallback
		if len(toolName) > 0 {
			return strings.ToUpper(toolName[:1]) + toolName[1:]
		}
		return toolName
	}
}

// ---------------------------------------------------------------------------
// Agent Messages - Sent from async operations back to the TUI
// ---------------------------------------------------------------------------

// AgentStartMsg signals that agent processing has begun
type AgentStartMsg struct {
	Timestamp time.Time
}

// AgentChunkMsg contains a streaming text chunk from the agent
type AgentChunkMsg struct {
	Text       string
	ToolUse    *types.ToolUseBlock
	ToolResult *types.ToolResultBlock
	Timestamp  time.Time
}

// AgentDoneMsg signals that agent processing is complete
type AgentDoneMsg struct {
	FullResponse string
	ToolCalls    int
	Timestamp    time.Time
}

// AgentErrorMsg signals an error during agent processing
type AgentErrorMsg struct {
	Error     error
	Timestamp time.Time
}

// AgentConnectingMsg signals that we're establishing connection to LLM
type AgentConnectingMsg struct {
	Endpoint  string
	Timestamp time.Time
}

// ModelChangedMsg signals that the active model has changed.
// Handled in App.Update to ensure status bar reflects the change.
type ModelChangedMsg struct {
	Model string
}

// AgentToolStartMsg signals a tool is being invoked
type AgentToolStartMsg struct {
	ToolID       string
	ToolName     string
	DisplayName  string
	ActivityDesc string // Rich description of what the tool is doing
	Input        map[string]any
}

// AgentToolDoneMsg signals a tool has completed
type AgentToolDoneMsg struct {
	ToolID  string
	Success bool
	Output  string
}

// ---------------------------------------------------------------------------
// AgentQueryCmd creates a Bubble Tea command for async agent queries
// ---------------------------------------------------------------------------

// AgentQueryParams bundles all parameters needed for an agent query
type AgentQueryParams struct {
	Messages     []types.Message
	SystemPrompt string
	CanUseTool   func(string, map[string]any, tools.Context) (tools.PermissionDecision, error)
	ToolCtx      tools.Context
	Client       llm.Client
}

// AgentQueryCmd returns a tea.Cmd that runs the agent query asynchronously
func AgentQueryCmd(params AgentQueryParams) tea.Cmd {
	return func() tea.Msg {
		// Create agent loop
		loop := agent.NewLoop(params.Client)

		// Build query params
		queryParams := agent.QueryParams{
			Messages:       params.Messages,
			SystemPrompt:   params.SystemPrompt,
			CanUseTool:     params.CanUseTool,
			ToolUseContext: params.ToolCtx,
		}

		// Execute query (this blocks in the goroutine, not the UI)
		stream, err := loop.Query(context.Background(), queryParams)
		if err != nil {
			return AgentErrorMsg{
				Error:     err,
				Timestamp: time.Now(),
			}
		}

		// We need to collect and return - but for true streaming,
		// we use a channel-based approach in the actual implementation
		var fullResponse strings.Builder
		toolCallCount := 0

		for event := range stream {
			switch e := event.(type) {
			case types.StreamMessage:
				for _, block := range e.Message.Content {
					switch b := block.(type) {
					case types.TextBlock:
						fullResponse.WriteString(b.Text)
					case types.ToolUseBlock:
						toolCallCount++
						// Note: In a full implementation, we'd send intermediate
						// messages for tool use. For now, we collect and return.
					}
				}
			}
		}

		return AgentDoneMsg{
			FullResponse: fullResponse.String(),
			ToolCalls:    toolCallCount,
			Timestamp:    time.Now(),
		}
	}
}

// ---------------------------------------------------------------------------
// Streaming Agent Query - True real-time with channel-based updates
// ---------------------------------------------------------------------------

// StreamingAgentQuery returns a command that sends updates via a channel
// This enables true real-time token-by-token display
func StreamingAgentQuery(params AgentQueryParams, updateChan chan<- tea.Msg) tea.Cmd {
	return func() tea.Msg {
		// Signal start
		updateChan <- AgentStartMsg{Timestamp: time.Now()}

		loop := agent.NewLoop(params.Client)
		queryParams := agent.QueryParams{
			Messages:       params.Messages,
			SystemPrompt:   params.SystemPrompt,
			CanUseTool:     params.CanUseTool,
			ToolUseContext: params.ToolCtx,
		}

		stream, err := loop.Query(context.Background(), queryParams)
		if err != nil {
			updateChan <- AgentErrorMsg{
				Error:     err,
				Timestamp: time.Now(),
			}
			return nil
		}

		var fullResponse strings.Builder
		toolCallCount := 0

		for event := range stream {
			switch e := event.(type) {
			case types.StreamMessage:
				for _, block := range e.Message.Content {
					switch b := block.(type) {
					case types.TextBlock:
						// Send each text chunk immediately for streaming display
						updateChan <- AgentChunkMsg{
							Text:      b.Text,
							Timestamp: time.Now(),
						}
						fullResponse.WriteString(b.Text)

					case types.ToolUseBlock:
						toolCallCount++
						updateChan <- AgentToolStartMsg{
							ToolID:      b.ID,
							ToolName:    b.Name,
							DisplayName: getToolDisplayName(b.Name),
							Input:       b.Input,
						}

					case types.ToolResultBlock:
						updateChan <- AgentToolDoneMsg{
							ToolID:  b.ToolUseID,
							Success: !b.IsError,
							Output:  fmt.Sprintf("%v", b.Content),
						}
					}
				}

			case types.ProgressMessage:
				// Tool progress updates
				updateChan <- AgentChunkMsg{
					Text:      fmt.Sprintf("→ %s", e.Data),
					Timestamp: time.Now(),
				}
			}
		}

		// Signal completion
		updateChan <- AgentDoneMsg{
			FullResponse: fullResponse.String(),
			ToolCalls:    toolCallCount,
			Timestamp:    time.Now(),
		}

		return nil
	}
}

// ---------------------------------------------------------------------------
// ChatAsyncUpdate - Helper for chat model to receive async updates
// ---------------------------------------------------------------------------

// ChatAsyncUpdate creates a command that waits for async updates
func ChatAsyncUpdate(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		select {
		case msg := <-ch:
			return msg
		case <-time.After(100 * time.Millisecond):
			// Timeout to prevent blocking - retry will be scheduled
			return nil
		}
	}
}

// ScheduleAsyncUpdate schedules the next async update check
func ScheduleAsyncUpdate(ch <-chan tea.Msg) tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		select {
		case msg := <-ch:
			return msg
		default:
			return nil
		}
	})
}
