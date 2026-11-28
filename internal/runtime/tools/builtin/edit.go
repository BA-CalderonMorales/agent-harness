package builtin

import (
	"fmt"
	"os"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools/fs"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// FileEditTool performs string-replace edits on files with stale-write protection.
var FileEditTool = tools.NewTool(tools.Tool{
	Name:        "edit",
	Description: "Edit a file by replacing one string with another. The old_string must match exactly. Supports replace_all for renaming across the file. Protected against concurrent modifications.",
	InputSchema: func() map[string]any {
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file_path":   map[string]any{"type": "string"},
				"old_string":  map[string]any{"type": "string"},
				"new_string":  map[string]any{"type": "string"},
				"replace_all": map[string]any{"type": "boolean", "description": "Replace all occurrences of old_string (default false)"},
			},
			"required": []string{"file_path", "old_string", "new_string"},
		}
	},
	Capabilities: tools.CapabilityFlags{
		IsEnabled:         func() bool { return true },
		IsConcurrencySafe: func(map[string]any) bool { return false }, // File edits are serial
		IsReadOnly:        func(map[string]any) bool { return false },
		IsDestructive:     func(map[string]any) bool { return false },
		InterruptBehavior: func() string { return "cancel" }, // Edits can be cancelled
	},
	ValidateInput: func(input map[string]any, ctx tools.Context) tools.ValidationResult {
		path := getString(input, "file_path")
		oldStr := getString(input, "old_string")
		if path == "" || oldStr == "" {
			return tools.ValidationResult{Valid: false, Message: "file_path and old_string are required"}
		}
		if _, err := requiredStringArgument(input, "new_string", "edit"); err != nil {
			return tools.ValidationResult{Valid: false, Message: err.Error() + "; supply the replacement text, or an explicit empty string for intentional deletion"}
		}
		// UNC path security
		if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, "//") {
			return tools.ValidationResult{Valid: false, Message: "UNC paths are not supported for security reasons"}
		}
		return tools.ValidationResult{Valid: true}
	},
	CheckPermissions: func(input map[string]any, ctx tools.Context) tools.PermissionDecision {
		return tools.PermissionDecision{Behavior: tools.Allow, UpdatedInput: input}
	},
	Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
		path := getString(input, "file_path")
		oldStr := getString(input, "old_string")
		newStr, inputErr := requiredStringArgument(input, "new_string", "edit")
		if inputErr != nil {
			return tools.ToolResult{}, inputErr
		}
		if path == "" || oldStr == "" {
			return tools.ToolResult{}, fmt.Errorf("file_path and old_string are required and must be nonempty strings")
		}
		replaceAll := false
		if v, ok := input["replace_all"].(bool); ok {
			replaceAll = v
		}

		// Preserve existing file mode (e.g., 0755 executable scripts).
		perm := os.FileMode(0644)
		if existingInfo, statErr := os.Stat(path); statErr == nil {
			perm = existingInfo.Mode().Perm()
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return tools.ToolResult{}, fmt.Errorf("failed to read file: %w", err)
		}

		content := string(data)

		// Normalize curly quotes for forgiving matches when exact match misses,
		// replacing only the matched span(s) in the original content so curly
		// quotes elsewhere in the file are preserved untouched.
		exactMatch := strings.Contains(content, oldStr)
		var fallbackUpdated string
		fallbackMatched := false
		if !exactMatch {
			fallbackUpdated, fallbackMatched = replaceWithNormalizedQuotes(content, oldStr, newStr, replaceAll)
		}

		if !exactMatch && !fallbackMatched {
			return tools.ToolResult{}, fmt.Errorf("old_string not found in file. The file may have changed — use the 'read' tool to see current contents, then try again with an exact match")
		}

		// Stale write protection: check if file was modified since last read
		if err := fs.DefaultStaleTracker.CheckStale(path); err != nil {
			return tools.ToolResult{}, fmt.Errorf("stale write detected: %w; the file may have been modified by another process - please re-read and try again", err)
		}

		var updated string
		if exactMatch {
			if replaceAll {
				updated = strings.ReplaceAll(content, oldStr, newStr)
			} else {
				updated = strings.Replace(content, oldStr, newStr, 1)
			}
		} else {
			updated = fallbackUpdated
		}

		// Write atomically: unique temp file then rename
		if err := writeFileAtomic(path, []byte(updated), perm); err != nil {
			return tools.ToolResult{}, fmt.Errorf("failed to write file: %w", err)
		}

		// Update stale tracker with new content
		info, _ := os.Stat(path)
		if info != nil {
			fs.DefaultStaleTracker.RecordRead(path, []byte(updated), info)
		}

		// Invalidate read cache for this file
		fs.DefaultCache.InvalidatePath(path)

		return tools.ToolResult{Data: fmt.Sprintf("Edited %s successfully", path)}, nil
	},
	MapResult: func(result any, toolUseID string) types.ToolResultBlock {
		content, _ := result.(string)
		return types.ToolResultBlock{ToolUseID: toolUseID, Content: content}
	},
	UserFacingName: func(map[string]any) string { return "edit" },
	GetActivityDescription: func(input map[string]any) string {
		if p, ok := input["file_path"].(string); ok {
			return "Editing " + p
		}
		return "Editing file"
	},
})

var quoteNormalizer = strings.NewReplacer(
	"\u2018", "'",
	"\u2019", "'",
	"\u201c", "\"",
	"\u201d", "\"",
)

// normalizeQuotes converts curly quotes to straight quotes for forgiving matches.
func normalizeQuotes(s string) string {
	return quoteNormalizer.Replace(s)
}

func normalizeQuoteRune(r rune) rune {
	switch r {
	case '\u2018', '\u2019':
		return '\''
	case '\u201c', '\u201d':
		return '"'
	default:
		return r
	}
}

// replaceWithNormalizedQuotes finds matches of oldStr in content after
// normalizing curly quotes, then replaces only the corresponding byte spans
// in the original content so all bytes outside the match(es) stay untouched.
func replaceWithNormalizedQuotes(content, oldStr, newStr string, replaceAll bool) (string, bool) {
	normOld := normalizeQuotes(oldStr)
	if normOld == "" {
		return content, false
	}
	var norm strings.Builder
	norm.Grow(len(content))
	normToOrig := make([]int, 0, len(content)+1)
	for origIdx, r := range content {
		nr := normalizeQuoteRune(r)
		start := norm.Len()
		norm.WriteRune(nr)
		for i := start; i < norm.Len(); i++ {
			normToOrig = append(normToOrig, origIdx)
		}
	}
	normToOrig = append(normToOrig, len(content))

	normContent := norm.String()
	var out strings.Builder
	out.Grow(len(content))
	searchFrom := 0
	lastOrig := 0
	matched := false
	for {
		rel := strings.Index(normContent[searchFrom:], normOld)
		if rel < 0 {
			break
		}
		matched = true
		matchStartNorm := searchFrom + rel
		matchEndNorm := matchStartNorm + len(normOld)
		origStart := normToOrig[matchStartNorm]
		origEnd := normToOrig[matchEndNorm]
		out.WriteString(content[lastOrig:origStart])
		out.WriteString(newStr)
		lastOrig = origEnd
		searchFrom = matchEndNorm
		if !replaceAll {
			break
		}
	}
	if !matched {
		return content, false
	}
	out.WriteString(content[lastOrig:])
	return out.String(), true
}
