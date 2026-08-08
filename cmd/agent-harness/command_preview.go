package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// buildCommandPreview generates a preview of what a tool will do.
func (app *App) buildCommandPreview(toolName string, toolInput map[string]any) string {
	switch toolName {
	case "write", "edit":
		path := getToolInputString(toolInput, "file_path")
		if path == "" {
			path = getToolInputString(toolInput, "path")
		}
		if path == "" {
			return ""
		}

		resolvedPath := path
		if !filepath.IsAbs(resolvedPath) {
			resolvedPath = filepath.Join(app.cwd, resolvedPath)
		}
		resolvedPath = filepath.Clean(resolvedPath)

		// Only generate previews for paths within the current workspace
		if !isPathInWorkspace(resolvedPath, app.cwd) {
			return fmt.Sprintf("Will modify: %s (outside workspace — preview unavailable)", resolvedPath)
		}

		// Check if file exists
		var preview string
		existing, err := os.ReadFile(resolvedPath)
		if err == nil && len(existing) > 0 {
			preview = fmt.Sprintf("Will overwrite: %s (%d bytes existing)", resolvedPath, len(existing))
			if toolName == "edit" && len(existing) > 0 {
				// Show first few lines of existing content
				lines := strings.Split(string(existing), "\n")
				maxLines := 5
				if len(lines) < maxLines {
					maxLines = len(lines)
				}
				preview += "\nExisting content (first " + fmt.Sprintf("%d", maxLines) + " lines):\n"
				for i := 0; i < maxLines; i++ {
					preview += lines[i] + "\n"
				}
			}
		} else {
			preview = fmt.Sprintf("Will create: %s", resolvedPath)
		}

		if toolName == "edit" {
			oldStr := getToolInputString(toolInput, "old_string")
			newStr := getToolInputString(toolInput, "new_string")
			if oldStr != "" {
				preview += "\nReplacing:\n  " + truncatePreviewLine(oldStr, 60)
			}
			if newStr != "" {
				preview += "\nWith:\n  " + truncatePreviewLine(newStr, 60)
			}
		} else {
			content := getToolInputString(toolInput, "content")
			if content != "" {
				lines := strings.Split(content, "\n")
				maxLines := 8
				if len(lines) < maxLines {
					maxLines = len(lines)
				}
				preview += "\nNew content (first " + fmt.Sprintf("%d", maxLines) + " lines):\n"
				for i := 0; i < maxLines; i++ {
					preview += lines[i] + "\n"
				}
				if len(lines) > maxLines {
					preview += fmt.Sprintf("... and %d more lines", len(lines)-maxLines)
				}
			}
		}
		return preview

	case "bash", "shell":
		cmd, _ := toolInput["command"].(string)
		if cmd == "" {
			return ""
		}
		// Risk assessment is handled by the dialog itself
		return ""

	default:
		return ""
	}
}

// getToolInputString safely extracts a string value from tool input.
func getToolInputString(toolInput map[string]any, key string) string {
	if v, ok := toolInput[key].(string); ok {
		return v
	}
	return ""
}

// isPathInWorkspace checks if a path is within the given workspace directory.
func isPathInWorkspace(path, workspace string) bool {
	workspaceAbs, err := filepath.Abs(workspace)
	if err != nil {
		return false
	}
	resolvedWorkspace, err := filepath.EvalSymlinks(workspaceAbs)
	if err != nil {
		return false
	}

	pathToCheck := path
	if !filepath.IsAbs(pathToCheck) {
		pathToCheck = filepath.Join(workspaceAbs, pathToCheck)
	}
	pathToCheck = filepath.Clean(pathToCheck)

	absPath, err := filepath.Abs(pathToCheck)
	if err != nil {
		return false
	}

	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return false
		}

		resolvedCandidate, candidateErr := resolvePathViaExistingAncestor(absPath)
		if candidateErr != nil {
			return false
		}
		if !isResolvedPathInWorkspace(resolvedWorkspace, resolvedCandidate) {
			return false
		}
		resolvedPath = resolvedCandidate
	}

	return isResolvedPathInWorkspace(resolvedWorkspace, resolvedPath)
}

// isResolvedPathInWorkspace checks whether resolvedPath remains within resolvedWorkspace.
// Both inputs are expected to be absolute paths with symlinks already resolved.
func isResolvedPathInWorkspace(resolvedWorkspace, resolvedPath string) bool {
	rel, err := filepath.Rel(resolvedWorkspace, resolvedPath)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

// resolvePathViaExistingAncestor resolves symlinks using the nearest existing ancestor of absPath.
func resolvePathViaExistingAncestor(absPath string) (string, error) {
	current := absPath
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			rel, relErr := filepath.Rel(current, absPath)
			if relErr != nil {
				return "", relErr
			}
			if rel == "." {
				return resolved, nil
			}
			return filepath.Join(resolved, rel), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		current = parent
	}
}

// truncatePreviewLine truncates a single line for preview display.
func truncatePreviewLine(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// extractCommandForDisplay extracts command string from tool input for display.
func (app *App) extractCommandForDisplay(toolName string, toolInput map[string]any) string {
	switch toolName {
	case "bash", "shell":
		if cmd, ok := toolInput["command"].(string); ok {
			return cmd
		}
	case "write", "edit":
		return extractWriteEditDisplay(toolInput)
	default:
		return extractGenericDisplay(toolInput, toolName)
	}
	return sprintf("[%s]", toolName)
}

// extractWriteEditDisplay extracts display for write/edit tools.
func extractWriteEditDisplay(toolInput map[string]any) string {
	var parts []string
	path := getToolInputString(toolInput, "file_path")
	if path == "" {
		path = getToolInputString(toolInput, "path")
	}
	if path != "" {
		parts = append(parts, path)
	}
	content := getToolInputString(toolInput, "content")
	if content != "" {
		lines := strings.Split(content, "\n")
		if len(lines) > 0 {
			display := lines[0]
			if len(display) > 50 {
				display = display[:47] + "..."
			}
			parts = append(parts, display)
		}
	}
	return strings.Join(parts, " - ")
}

// extractGenericDisplay extracts display for generic tools.
func extractGenericDisplay(toolInput map[string]any, toolName string) string {
	var parts []string
	for k, v := range toolInput {
		if k != "command" && k != "content" {
			parts = append(parts, sprintf("%s=%v", k, v))
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, ", ")
	}
	return sprintf("[%s]", toolName)
}

// requestCommandApproval requests user approval for a command.

// checkDestructive determines if a command is destructive.
func checkDestructive(toolName, command string) bool {
	if toolName == "bash" || toolName == "shell" {
		if strings.Contains(command, "rm ") || strings.Contains(command, "dd ") {
			return true
		}
	}
	return toolName == "write" || toolName == "edit"
}

// interactiveSetup guides user through initial configuration.
