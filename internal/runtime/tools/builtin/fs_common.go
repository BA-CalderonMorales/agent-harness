package builtin

import "strings"

// commonIgnoredDirs are directories that should be skipped during filesystem
// walks to keep tool execution fast on large repositories.
var commonIgnoredDirs = []string{
	".git",
	"node_modules",
	".venv",
	"venv",
	"target",
	"vendor",
	"__pycache__",
	".cache",
	"dist",
	"build",
	".idea",
	".vscode",
	"*.egg-info",
}

// isCommonIgnoredDir reports whether a directory name should be skipped
// during recursive filesystem traversal.
func isCommonIgnoredDir(name string) bool {
	for _, d := range commonIgnoredDirs {
		if strings.EqualFold(name, d) {
			return true
		}
	}
	return false
}
