// Package defaults provides hardcoded configuration values for all buckets.
// These are the source of truth for default settings - not in individual bucket files.
package defaults

// FileSystem bucket defaults
const (
	// File size limits
	FSMaxFileSize     = 10 * 1024 * 1024 // 10MB
	FSMaxFileSizeText = "10MB"

	// Read limits
	FSMaxReadOffset = 10000
	FSMaxReadLimit  = 1000

	// Path restrictions
	FSDefaultBlockedPaths = "/dev/zero,/dev/random,/dev/urandom,/dev/stdin"
)

// FSAllowedExtensions contains safe file extensions for editing
var FSAllowedExtensions = []string{
	".go", ".js", ".ts", ".jsx", ".tsx",
	".py", ".rb", ".rs", ".java", ".kt",
	".c", ".cpp", ".h", ".hpp",
	".md", ".txt", ".json", ".yaml", ".yml",
	".html", ".css", ".scss", ".sass",
	".sh", ".bash", ".zsh",
	".xml", ".toml", ".ini", ".conf",
}

// FSDangerousPaths contains paths that should never be accessed
var FSDangerousPaths = []string{
	"/dev/zero",
	"/dev/random",
	"/dev/urandom",
	"/dev/stdin",
	"/dev/sda",
	"/dev/sdb",
	"/etc/shadow",
	"/etc/passwd",
	"/.ssh/id_rsa",
	"/.ssh/id_ed25519",
	".env",
	".env.local",
	".env.production",
}

// FSTextMimeTypes maps extensions to MIME types for text files
var FSTextMimeTypes = map[string]string{
	".go":   "text/x-go",
	".js":   "application/javascript",
	".ts":   "application/typescript",
	".py":   "text/x-python",
	".md":   "text/markdown",
	".json": "application/json",
	".yaml": "application/yaml",
	".yml":  "application/yaml",
	".txt":  "text/plain",
}
