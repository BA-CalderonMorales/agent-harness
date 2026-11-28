package builtin

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

const (
	maxGrepMatches   = 500
	maxGrepLineBytes = 1 << 20
)

var errGrepLimitReached = errors.New("grep match limit reached")

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
		truncated := false
		info, err := os.Stat(searchPath)
		if err != nil {
			return tools.ToolResult{}, err
		}

		if info.IsDir() {
			cleanRoot := filepath.Clean(searchPath)
			walkErr := filepath.WalkDir(searchPath, func(path string, d iofs.DirEntry, err error) error {
				if ctx.AbortController != nil && ctx.AbortController.Err() != nil {
					return ctx.AbortController.Err()
				}
				if err != nil {
					return nil
				}
				if d.IsDir() {
					if filepath.Clean(path) != cleanRoot && isCommonIgnoredDir(d.Name()) {
						return filepath.SkipDir
					}
					return nil
				}
				if !d.Type().IsRegular() {
					return nil
				}
				if include != "" {
					matched, _ := filepath.Match(include, filepath.Base(path))
					if !matched {
						return nil
					}
				}
				remaining := maxGrepMatches - len(matches)
				if remaining <= 0 {
					truncated = true
					return errGrepLimitReached
				}
				lines, hitLimit, grepErr := grepFile(ctx.AbortController, path, re, remaining)
				if grepErr != nil {
					return grepErr
				}
				if len(lines) > 0 {
					matches = append(matches, lines...)
				}
				if hitLimit {
					truncated = true
					return errGrepLimitReached
				}
				return nil
			})
			if walkErr != nil && !errors.Is(walkErr, errGrepLimitReached) {
				return tools.ToolResult{}, walkErr
			}
		} else {
			var grepErr error
			matches, truncated, grepErr = grepFile(ctx.AbortController, searchPath, re, maxGrepMatches)
			if grepErr != nil {
				return tools.ToolResult{}, grepErr
			}
		}

		result := strings.Join(matches, "\n")
		if truncated {
			result += fmt.Sprintf("\n\n[Output truncated: reached %d match limit; narrow the pattern or path]", maxGrepMatches)
		}
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
		if r := []rune(pattern); len(r) > 20 {
			pattern = string(r[:17]) + "..."
		}
		return fmt.Sprintf("Searching for '%s' in %s", pattern, name)
	},
	GetToolUseSummary: func(input map[string]any) string {
		pattern := getString(input, "pattern")
		if r := []rune(pattern); len(r) > 30 {
			pattern = string(r[:27]) + "..."
		}
		return pattern
	},
})

func grepFile(ctx context.Context, path string, re *regexp.Regexp, limit int) ([]string, bool, error) {
	if ctx != nil && ctx.Err() != nil {
		return nil, false, ctx.Err()
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, false, nil
	}
	defer func() { _ = f.Close() }()

	// Skip binary files by checking the first 512 bytes for a NUL byte.
	var probe [512]byte
	n, readErr := f.Read(probe[:])
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return nil, false, nil
	}
	if bytes.IndexByte(probe[:n], 0) >= 0 {
		return nil, false, nil
	}

	reader := io.MultiReader(bytes.NewReader(probe[:n]), f)
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), maxGrepLineBytes)

	var matches []string
	lineNum := 1
	for scanner.Scan() {
		if ctx != nil && ctx.Err() != nil {
			return matches, false, ctx.Err()
		}
		line := scanner.Text()
		if re.MatchString(line) {
			if limit > 0 && len(matches) >= limit {
				return matches, true, nil
			}
			matches = append(matches, fmt.Sprintf("%s:%d:%s", path, lineNum, line))
		}
		lineNum++
	}
	return matches, false, nil
}
