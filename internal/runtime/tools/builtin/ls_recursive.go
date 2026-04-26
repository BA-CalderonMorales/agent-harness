package builtin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// LsRecursiveTool lists files recursively in a directory.
var LsRecursiveTool = tools.NewTool(tools.Tool{
	Name:        "ls_recursive",
	Description: "List files and directories recursively in a specified path.",
	InputSchema: func() map[string]any {
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":    map[string]any{"type": "string", "description": "The directory to list"},
				"depth":   map[string]any{"type": "integer", "description": "Maximum recursion depth", "default": 2},
				"exclude": map[string]any{"type": "string", "description": "Glob pattern to exclude (e.g. node_modules/*)"},
			},
			"required": []string{"path"},
		}
	},
	Capabilities: tools.CapabilityFlags{
		IsEnabled:         func() bool { return true },
		IsConcurrencySafe: func(input map[string]any) bool { return true },
		IsReadOnly:        func(map[string]any) bool { return true },
		IsDestructive:     func(map[string]any) bool { return false },
		InterruptBehavior: func() string { return "cancel" },
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
	Call: func(input map[string]any, ctx tools.Context, canUseTool tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
		root := input["path"].(string)
		maxDepth := 2
		if d, ok := getNumber(input, "depth"); ok {
			maxDepth = int(d)
		}
		exclude, _ := input["exclude"].(string)

		const maxResults = 200
		var results []string
		var dirs, files int
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip common ignored directories for performance
			if info.IsDir() && isCommonIgnoredDir(info.Name()) {
				return filepath.SkipDir
			}

			// Check depth
			rel, _ := filepath.Rel(root, path)
			if rel != "." && strings.Count(rel, string(os.PathSeparator)) >= maxDepth {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// Check exclude
			if exclude != "" {
				if matched, _ := filepath.Match(exclude, info.Name()); matched {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}

			if info.IsDir() {
				dirs++
			} else {
				files++
			}
			results = append(results, path)
			if onProgress != nil {
				onProgress("found: " + path)
			}
			return nil
		})

		if err != nil {
			return tools.ToolResult{Data: "error: " + err.Error()}, nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Recursive listing: %s (depth=%d)\n", root, maxDepth)
		fmt.Fprintf(&b, "Found %d directories, %d files\n\n", dirs, files)

		truncated := false
		if len(results) > maxResults {
			results = results[:maxResults]
			truncated = true
		}
		for _, r := range results {
			b.WriteString(r)
			b.WriteByte('\n')
		}
		if truncated {
			fmt.Fprintf(&b, "\n(... %d more entries not shown ...)", dirs+files-maxResults)
		}

		return tools.ToolResult{Data: b.String()}, nil
	},
	MapResult: func(result any, toolUseID string) types.ToolResultBlock {
		content, _ := result.(string)
		return types.ToolResultBlock{ToolUseID: toolUseID, Content: content}
	},
	UserFacingName: func(map[string]any) string { return "ls_recursive" },
	GetActivityDescription: func(input map[string]any) string {
		p, _ := input["path"].(string)
		return fmt.Sprintf("Listing %s recursively", p)
	},
})
