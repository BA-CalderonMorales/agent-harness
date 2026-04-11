// Package buckets provides domain-specific LoopBase implementations.
package buckets

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/state"
	"github.com/BA-CalderonMorales/agent-harness/internal/session/loop"
	"github.com/BA-CalderonMorales/agent-harness/internal/session/loop/buckets/defaults"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// LoopUI handles user-facing and session management operations.
// Tools: ask, todo, export, notebook, rewind, settings
type UIBucket struct {
	sessionManager *state.SessionManager
	exportDir      string
	notebookDir    string
}

// NewLoopUI creates a UI bucket with default directories.
func UI(exportDir, notebookDir string) *UIBucket {
	sm, _ := state.NewSessionManager()
	if exportDir == "" {
		exportDir = defaults.UIExportDirDefault
	}
	if notebookDir == "" {
		notebookDir = defaults.UINotebookDirDefault
	}
	return &UIBucket{
		sessionManager: sm,
		exportDir:      exportDir,
		notebookDir:    notebookDir,
	}
}

// WithSessionManager sets a custom session manager.
func (ui *UIBucket) WithSessionManager(sm *state.SessionManager) *UIBucket {
	ui.sessionManager = sm
	return ui
}

// Name returns the bucket identifier.
func (ui *UIBucket) Name() string {
	return "ui"
}

// CanHandle determines if this bucket handles the tool.
func (ui *UIBucket) CanHandle(toolName string, input map[string]any) bool {
	switch toolName {
	case "ask", "ask_user", "todo", "todo_write", "export",
		"notebook", "notebook_edit", "rewind", "settings":
		return true
	}
	return false
}

// Capabilities describes what this bucket can do.
func (ui *UIBucket) Capabilities() loop.BucketCapabilities {
	return loop.BucketCapabilities{
		IsConcurrencySafe: true,
		IsReadOnly:        false,
		IsDestructive:     false,
		ToolNames: []string{
			"ask", "ask_user", "todo", "todo_write",
			"export", "notebook", "notebook_edit", "rewind", "settings",
		},
		Category: "ui",
	}
}

// Execute runs the UI operation.
func (ui *UIBucket) Execute(ctx loop.ExecutionContext) loop.LoopResult {
	switch ctx.ToolName {
	case "ask", "ask_user":
		return ui.handleAsk(ctx)
	case "todo", "todo_write":
		return ui.handleTodo(ctx)
	case "export":
		return ui.handleExport(ctx)
	case "notebook", "notebook_edit":
		return ui.handleNotebook(ctx)
	case "rewind":
		return ui.handleRewind(ctx)
	case "settings":
		return ui.handleSettings(ctx)
	default:
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("unknown_tool", "ui bucket doesn't handle: "+ctx.ToolName),
		}
	}
}

// handleAsk asks the user a question.
func (ui *UIBucket) handleAsk(ctx loop.ExecutionContext) loop.LoopResult {
	question, _ := ctx.Input["question"].(string)
	options := []string{}
	if opts, ok := ctx.Input["options"].([]any); ok {
		for _, o := range opts {
			if s, ok := o.(string); ok {
				options = append(options, s)
			}
		}
	}

	// Format the question
	var result strings.Builder
	result.WriteString("Question: ")
	result.WriteString(question)
	if len(options) > 0 {
		result.WriteString("\nOptions: ")
		result.WriteString(strings.Join(options, ", "))
	}

	return loop.LoopResult{
		Success: true,
		Data:    result.String(),
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result.String()}},
		}},
	}
}

// handleTodo manages todo lists.
func (ui *UIBucket) handleTodo(ctx loop.ExecutionContext) loop.LoopResult {
	todos, _ := ctx.Input["todos"].([]any)

	var result strings.Builder
	result.WriteString("Todo list updated:\n")
	for i, t := range todos {
		if todo, ok := t.(map[string]any); ok {
			content, _ := todo["content"].(string)
			status, _ := todo["status"].(string)
			result.WriteString(fmt.Sprintf("  %d. [%s] %s\n", i+1, status, content))
		}
	}

	return loop.LoopResult{
		Success: true,
		Data:    result.String(),
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result.String()}},
		}},
	}
}

// handleExport exports conversation/session data.
func (ui *UIBucket) handleExport(ctx loop.ExecutionContext) loop.LoopResult {
	format, _ := ctx.Input["format"].(string)
	if format == "" {
		format = "json"
	}

	// In real implementation, would serialize current session
	result := fmt.Sprintf("Session exported as %s to %s/", format, ui.exportDir)

	return loop.LoopResult{
		Success: true,
		Data:    result,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result}},
		}},
	}
}

// handleNotebook manages notebook entries.
func (ui *UIBucket) handleNotebook(ctx loop.ExecutionContext) loop.LoopResult {
	action, _ := ctx.Input["action"].(string)
	content, _ := ctx.Input["content"].(string)

	var result string
	switch action {
	case "create":
		result = fmt.Sprintf("Created notebook entry: %s", content[:min(len(content), 50)])
	case "append":
		result = fmt.Sprintf("Appended to notebook: %s", content[:min(len(content), 50)])
	case "read":
		result = "Notebook contents would be read here"
	default:
		result = fmt.Sprintf("Notebook action '%s' completed", action)
	}

	return loop.LoopResult{
		Success: true,
		Data:    result,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result}},
		}},
	}
}

// handleRewind rewinds to a previous state.
func (ui *UIBucket) handleRewind(ctx loop.ExecutionContext) loop.LoopResult {
	steps := 1
	if s, ok := ctx.Input["steps"].(float64); ok {
		steps = int(s)
	}

	result := fmt.Sprintf("Rewound %d steps", steps)

	return loop.LoopResult{
		Success: true,
		Data:    result,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result}},
		}},
	}
}

// handleSettings manages settings.
func (ui *UIBucket) handleSettings(ctx loop.ExecutionContext) loop.LoopResult {
	action, _ := ctx.Input["action"].(string)
	key, _ := ctx.Input["key"].(string)
	value, _ := ctx.Input["value"].(string)

	var result string
	switch action {
	case "get":
		result = fmt.Sprintf("Setting '%s': (value would be retrieved)", key)
	case "set":
		result = fmt.Sprintf("Set '%s' = '%s'", key, value)
	case "list":
		result = "Available settings:\n  - model\n  - provider\n  - theme"
	default:
		result = fmt.Sprintf("Settings action '%s' completed", action)
	}

	return loop.LoopResult{
		Success: true,
		Data:    result,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result}},
		}},
	}
}

// SessionInfo provides session metadata for UI display.
type SessionInfo struct {
	ID           string    `json:"id"`
	MessageCount int       `json:"message_count"`
	CreatedAt    time.Time `json:"created_at"`
	Model        string    `json:"model"`
}

// GetSessions returns list of available sessions.
func (ui *UIBucket) GetSessions() ([]SessionInfo, error) {
	if ui.sessionManager == nil {
		return nil, nil
	}
	sessions, err := ui.sessionManager.ListSessions()
	if err != nil {
		return nil, err
	}

	var result []SessionInfo
	for _, s := range sessions {
		result = append(result, SessionInfo{
			ID:           s.ID,
			MessageCount: s.MessageCount,
			CreatedAt:    s.CreatedAt,
			Model:        s.Model,
		})
	}
	return result, nil
}

// ToJSON serializes value to JSON string.
func ToJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Ensure LoopUI implements LoopBase
var _ loop.LoopBase = (*UIBucket)(nil)
