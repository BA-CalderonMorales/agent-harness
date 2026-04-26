package builtin

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// GrepTool searches file contents for a regex pattern.
var GrepTool = tools.NewTool(tools.Tool{
	Name:        "grep",
	Description: "Search file contents using a regular expression.",
	InputSchema: func() map[string]any {
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{"type": "string"},
				"path":    map[string]any{"type": "string", "description": "File or directory to search"},
				"include": map[string]any{"type": "string", "description": "Glob pattern for files to include"},
			},
			"required": []string{"pattern", "path"},
		}
	},
	Capabilities: tools.CapabilityFlags{
		IsEnabled:             func() bool { return true },
		IsConcurrencySafe:     func(map[string]any) bool { return true },
		IsReadOnly:            func(map[string]any) bool { return true },
		IsSearchOrReadCommand: func(map[string]any) tools.SearchReadFlags { return tools.SearchReadFlags{IsSearch: true} },
	},
	ValidateInput: func(input map[string]any, ctx tools.Context) tools.ValidationResult {
		if getString(input, "pattern") == "" || getString(input, "path") == "" {
			return tools.ValidationResult{Valid: false, Message: "pattern and path are required"}
		}
		return tools.ValidationResult{Valid: true}
	},
	CheckPermissions: func(input map[string]any, ctx tools.Context) tools.PermissionDecision {
		return tools.PermissionDecision{Behavior: tools.Allow, UpdatedInput: input}
	},
	Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
		pattern := getString(input, "pattern")
		searchPath := getString(input, "path")
		include := getString(input, "include")

		re, err := regexp.Compile(pattern)
		if err != nil {
			return tools.ToolResult{}, fmt.Errorf("invalid regex: %w", err)
		}

		var matches []string
		info, err := os.Stat(searchPath)
		if err != nil {
			return tools.ToolResult{}, err
		}

		if info.IsDir() {
			_ = filepath.Walk(searchPath, func(path string, fi os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if fi.IsDir() {
					if isCommonIgnoredDir(fi.Name()) {
						return filepath.SkipDir
					}
					return nil
				}
				if include != "" {
					matched, _ := filepath.Match(include, filepath.Base(path))
					if !matched {
						return nil
					}
				}
				if lines := grepFile(path, re); len(lines) > 0 {
					matches = append(matches, lines...)
				}
				return nil
			})
		} else {
			matches = grepFile(searchPath, re)
		}

		result := strings.Join(matches, "\n")
		if result == "" {
			result = "(no matches found)"
		}
		return tools.ToolResult{Data: result}, nil
	},
	MapResult: func(result any, toolUseID string) types.ToolResultBlock {
		return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
	},
	UserFacingName: func(map[string]any) string { return "Grep" },
	GetActivityDescription: func(input map[string]any) string {
		pattern := getString(input, "pattern")
		path := getString(input, "path")
		if pattern == "" {
			return "Searching code"
		}
		// Show just the filename or dir name
		parts := strings.Split(path, "/")
		name := parts[len(parts)-1]
		if len(pattern) > 20 {
			pattern = pattern[:17] + "..."
		}
		return fmt.Sprintf("Searching for '%s' in %s", pattern, name)
	},
	GetToolUseSummary: func(input map[string]any) string {
		pattern := getString(input, "pattern")
		if len(pattern) > 30 {
			pattern = pattern[:27] + "..."
		}
		return pattern
	},
})

func grepFile(path string, re *regexp.Regexp) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var matches []string
	scanner := bufio.NewScanner(f)
	lineNum := 1
	for scanner.Scan() {
		line := scanner.Text()
		if re.MatchString(line) {
			matches = append(matches, fmt.Sprintf("%s:%d:%s", path, lineNum, line))
		}
		lineNum++
	}
	return matches
}
