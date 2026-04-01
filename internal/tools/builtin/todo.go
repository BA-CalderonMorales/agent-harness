package builtin

import (
	"fmt"
	"sync"

	"github.com/BA-CalderonMorales/agent-harness/internal/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// TodoItem represents a single todo entry.
type TodoItem struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Status string `json:"status"`
}

// Global todo store (in production, this belongs in state).
var (
	todoStore []TodoItem
	todoMu    sync.RWMutex
)

// TodoWriteTool manages a simple todo list.
var TodoWriteTool = tools.NewTool(tools.Tool{
	Name:        "todo_write",
	Description: "Create or update a todo list. Use this to track steps in a plan.",
	InputSchema: func() map[string]any {
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"todos": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id":     map[string]any{"type": "string"},
							"text":   map[string]any{"type": "string"},
							"status": map[string]any{"type": "string", "enum": []string{"pending", "done", "cancelled"}},
						},
						"required": []string{"id", "text", "status"},
					},
				},
			},
			"required": []string{"todos"},
		}
	},
	Capabilities: tools.CapabilityFlags{
		IsEnabled:         func() bool { return true },
		IsConcurrencySafe: func(map[string]any) bool { return true },
		IsReadOnly:        func(map[string]any) bool { return false },
	},
	ValidateInput: func(input map[string]any, ctx tools.Context) tools.ValidationResult {
		if _, ok := input["todos"].([]any); !ok {
			return tools.ValidationResult{Valid: false, Message: "todos array is required"}
		}
		return tools.ValidationResult{Valid: true}
	},
	CheckPermissions: func(input map[string]any, ctx tools.Context) tools.PermissionDecision {
		return tools.PermissionDecision{Behavior: tools.Allow, UpdatedInput: input}
	},
	Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
		rawTodos := input["todos"].([]any)
		newTodos := make([]TodoItem, 0, len(rawTodos))
		for _, r := range rawTodos {
			m, ok := r.(map[string]any)
			if !ok {
				continue
			}
			newTodos = append(newTodos, TodoItem{
				ID:     getString(m, "id"),
				Text:   getString(m, "text"),
				Status: getString(m, "status"),
			})
		}

		todoMu.Lock()
		todoStore = newTodos
		todoMu.Unlock()

		return tools.ToolResult{Data: fmt.Sprintf("Updated %d todos", len(newTodos))}, nil
	},
	MapResult: func(result any, toolUseID string) types.ToolResultBlock {
		content, _ := result.(string)
		return types.ToolResultBlock{ToolUseID: toolUseID, Content: content}
	},
	UserFacingName: func(map[string]any) string { return "todo" },
})
