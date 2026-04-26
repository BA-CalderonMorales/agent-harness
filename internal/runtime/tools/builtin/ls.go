package builtin

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// ListDirectoryTool lists the contents of a single directory (non-recursive).
var ListDirectoryTool = tools.NewTool(tools.Tool{
	Name:        "ls",
	Aliases:     []string{"list_directory"},
	Description: "List the contents of a single directory with file types and sizes.",
	InputSchema: func() map[string]any {
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "The directory to list"},
			},
			"required": []string{"path"},
		}
	},
	Capabilities: tools.CapabilityFlags{
		IsEnabled:         func() bool { return true },
		IsConcurrencySafe: func(map[string]any) bool { return true },
		IsReadOnly:        func(map[string]any) bool { return true },
		IsDestructive:     func(map[string]any) bool { return false },
		InterruptBehavior: func() string { return "cancel" },
		IsSearchOrReadCommand: func(map[string]any) tools.SearchReadFlags {
			return tools.SearchReadFlags{IsList: true}
		},
	},
	ValidateInput: func(input map[string]any, ctx tools.Context) tools.ValidationResult {
		p, ok := input["path"].(string)
		if !ok || p == "" {
			return tools.ValidationResult{Valid: false, Message: "path is required"}
		}
		return tools.ValidationResult{Valid: true}
	},
	CheckPermissions: func(input map[string]any, ctx tools.Context) tools.PermissionDecision {
		return tools.PermissionDecision{Behavior: tools.Allow, UpdatedInput: input}
	},
	Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
		root := input["path"].(string)

		entries, err := os.ReadDir(root)
		if err != nil {
			return tools.ToolResult{Data: "error: " + err.Error()}, nil
		}

		var lines []string
		var dirs, files int

		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				lines = append(lines, fmt.Sprintf("%s  ?", entry.Name()))
				continue
			}

			marker := "F"
			if entry.IsDir() {
				marker = "D"
				dirs++
			} else {
				files++
			}

			size := info.Size()
			lines = append(lines, fmt.Sprintf("%-1s %8d  %s", marker, size, entry.Name()))
		}

		sort.Strings(lines)

		header := fmt.Sprintf("Listing: %s", root)
		footer := fmt.Sprintf("\n%d entries (%d directories, %d files)", len(lines), dirs, files)
		result := header + "\n" + strings.Join(lines, "\n") + footer

		return tools.ToolResult{Data: result}, nil
	},
	MapResult: func(result any, toolUseID string) types.ToolResultBlock {
		content, _ := result.(string)
		return types.ToolResultBlock{ToolUseID: toolUseID, Content: content}
	},
	UserFacingName: func(map[string]any) string { return "ls" },
	GetActivityDescription: func(input map[string]any) string {
		p, _ := input["path"].(string)
		return fmt.Sprintf("Listing directory %s", p)
	},
})
