package builtin

import (
	"path/filepath"
	"strings"
)

// commonIgnoredDirs are directories that should be skipped during filesystem
// walks to keep tool execution fast on large repositories.
var commonIgnoredDirs = []string{
	".git",
	".claude",
	".worktrees",
	"node_modules",
	".venv",
	"venv",
	"target",
	"vendor",
	"__pycache__",
	".cache",
	".next",
	".nuxt",
	"dist",
	"build",
	".idea",
	".vscode",
	"*.egg-info",
}

// isCommonIgnoredDir reports whether a directory name should be skipped
// during recursive filesystem traversal.
func isCommonIgnoredDir(name string) bool {
	lower := strings.ToLower(name)
	for _, d := range commonIgnoredDirs {
		if strings.ContainsAny(d, "*?[") {
			if matched, err := filepath.Match(strings.ToLower(d), lower); err == nil && matched {
				return true
			}
			continue
		}
		if strings.EqualFold(name, d) {
			return true
		}
	}
	return false
}
