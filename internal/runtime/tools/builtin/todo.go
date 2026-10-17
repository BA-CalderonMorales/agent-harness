package builtin

import (
	"fmt"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// todoStatuses are the states a plan item may hold. in_progress exists so
// the plan can say which step is running: without it every item was either
// pending or done, and the checklist the user reads could never show where
// the work actually was.
var todoStatuses = []string{"pending", "in_progress", "done", "cancelled"}

func validTodoStatus(status string) bool {
	for _, s := range todoStatuses {
		if s == status {
			return true
		}
	}
	return false
}

// TodoItem represents a single todo entry.
type TodoItem struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Status string `json:"status"`
}

// TodoWriteTool manages a simple todo list.
var TodoWriteTool = tools.NewTool(tools.Tool{
	Name:        "todo_write",
	Description: "Create or replace the task plan. Call this BEFORE the first mutating tool call on any task that needs more than one step, so the user can see what you intend to do before you start changing things. Keep exactly one item in_progress and mark items done as you finish them.",
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
							"status": map[string]any{"type": "string", "enum": todoStatuses},
						},
						"required": []string{"text", "status"},
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
		raw, ok := input["todos"].([]any)
		if !ok {
			return tools.ValidationResult{Valid: false, Message: "todos array is required"}
		}
		for i, entry := range raw {
			m, ok := entry.(map[string]any)
			if !ok {
				return tools.ValidationResult{Valid: false, Message: fmt.Sprintf("todo %d is not an object", i)}
			}
			if strings.TrimSpace(getString(m, "text")) == "" {
				return tools.ValidationResult{Valid: false, Message: fmt.Sprintf("todo %d has no text", i)}
			}
			// An unrecognized status used to be accepted and silently
			// rendered as pending: a plan that said done read as not
			// started. Reject it and name the allowed values.
			status := getString(m, "status")
			if !validTodoStatus(status) {
				return tools.ValidationResult{
					Valid: false,
					Message: fmt.Sprintf("todo %d has status %q; allowed values: %s",
						i, status, strings.Join(todoStatuses, ", ")),
				}
			}
		}
		return tools.ValidationResult{Valid: true}
	},
	CheckPermissions: func(input map[string]any, ctx tools.Context) tools.PermissionDecision {
		return tools.PermissionDecision{Behavior: tools.Allow, UpdatedInput: input}
	},
	Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
		rawTodos := input["todos"].([]any)
		newTodos := make([]TodoItem, 0, len(rawTodos))
		for i, r := range rawTodos {
			m, ok := r.(map[string]any)
			if !ok {
				continue
			}
			id := getString(m, "id")
			if id == "" {
				// An id the checklist never shows is not worth blocking on;
				// the position is a stable identity within one write.
				id = fmt.Sprintf("%d", i+1)
			}
			newTodos = append(newTodos, TodoItem{
				ID:     id,
				Text:   getString(m, "text"),
				Status: getString(m, "status"),
			})
		}

		return tools.ToolResult{Data: renderTodoSummary(newTodos)}, nil
	},
	MapResult: func(result any, toolUseID string) types.ToolResultBlock {
		content, _ := result.(string)
		return types.ToolResultBlock{ToolUseID: toolUseID, Content: content}
	},
	UserFacingName: func(map[string]any) string { return "todo" },
})

// renderTodoSummary echoes the plan back to the model. The checklist is
// the plan the user reads, so the model has to see the same list it just
// wrote: a plan it cannot read back is a plan it will let drift.
func renderTodoSummary(todos []TodoItem) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Updated %d todos", len(todos))
	for _, td := range todos {
		fmt.Fprintf(&b, "\n  [%s] %s", td.Status, td.Text)
	}
	return b.String()
}
