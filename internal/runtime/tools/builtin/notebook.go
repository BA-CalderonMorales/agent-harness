package builtin

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// NotebookCell represents a single cell in a Jupyter notebook.
type NotebookCell struct {
	CellType string         `json:"cell_type"`
	Source   []string       `json:"source"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Notebook represents a Jupyter notebook structure.
type Notebook struct {
	Cells    []NotebookCell `json:"cells"`
	Metadata map[string]any `json:"metadata"`
	NbFormat int            `json:"nbformat"`
	NbMinor  int            `json:"nbformat_minor"`
}

// NotebookEditTool edits Jupyter notebooks.
var NotebookEditTool = tools.NewTool(tools.Tool{
	Name:        "notebook_edit",
	Description: "Edit a Jupyter notebook cell by replacing its source content.",
	InputSchema: func() map[string]any {
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"notebook_path": map[string]any{"type": "string"},
				"cell_index":    map[string]any{"type": "integer"},
				"new_source":    map[string]any{"type": "string"},
			},
			"required": []string{"notebook_path", "cell_index", "new_source"},
		}
	},
	Capabilities: tools.CapabilityFlags{
		IsEnabled:         func() bool { return true },
		IsConcurrencySafe: func(map[string]any) bool { return false },
		IsReadOnly:        func(map[string]any) bool { return false },
	},
	ValidateInput: func(input map[string]any, ctx tools.Context) tools.ValidationResult {
		if getString(input, "notebook_path") == "" {
			return tools.ValidationResult{Valid: false, Message: "notebook_path is required"}
		}
		return tools.ValidationResult{Valid: true}
	},
	CheckPermissions: func(input map[string]any, ctx tools.Context) tools.PermissionDecision {
		return tools.PermissionDecision{Behavior: tools.Allow, UpdatedInput: input}
	},
	Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
		path := getString(input, "notebook_path")
		cellIndex := getInt(input, "cell_index")
		newSource := getString(input, "new_source")

		data, err := os.ReadFile(path)
		if err != nil {
			return tools.ToolResult{}, fmt.Errorf("failed to read notebook: %w", err)
		}

		var nb Notebook
		if err := json.Unmarshal(data, &nb); err != nil {
			return tools.ToolResult{}, fmt.Errorf("invalid notebook JSON: %w", err)
		}

		if cellIndex < 0 || cellIndex >= len(nb.Cells) {
			return tools.ToolResult{}, fmt.Errorf("cell index %d out of range", cellIndex)
		}

		nb.Cells[cellIndex].Source = []string{newSource}

		updated, err := json.MarshalIndent(nb, "", "  ")
		if err != nil {
			return tools.ToolResult{}, err
		}

		if err := os.WriteFile(path, updated, 0644); err != nil {
			return tools.ToolResult{}, fmt.Errorf("failed to write notebook: %w", err)
		}

		return tools.ToolResult{Data: fmt.Sprintf("Updated cell %d in %s", cellIndex, path)}, nil
	},
	MapResult: func(result any, toolUseID string) types.ToolResultBlock {
		return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
	},
	UserFacingName: func(map[string]any) string { return "notebook_edit" },
	GetActivityDescription: func(input map[string]any) string {
		if p, ok := input["notebook_path"].(string); ok {
			return "Editing notebook " + p
		}
		return "Editing notebook"
	},
})
