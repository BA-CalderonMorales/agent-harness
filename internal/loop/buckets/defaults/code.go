package defaults

// Code bucket defaults
const (
	// Lint settings
	CodeLintTimeoutSecs     = 60
	CodeLintMaxIssues       = 50
	CodeLintMaxFileSize     = 1024 * 1024 // 1MB

	// Format settings
	CodeFormatTimeoutSecs   = 30
	CodeFormatMaxFileSize   = 1024 * 1024 // 1MB

	// Analysis settings
	CodeAnalysisMaxDepth    = 10
	CodeAnalysisMaxFiles    = 100
)

// CodeLanguageTools maps languages to their tools
var CodeLanguageTools = map[string]CodeToolConfig{
	"go": {
		Linter:     "golangci-lint",
		Formatter:  "gofmt",
		Compiler:   "go build",
		TestRunner: "go test",
		Extensions: []string{".go"},
	},
	"javascript": {
		Linter:     "eslint",
		Formatter:  "prettier",
		Compiler:   "node --check",
		TestRunner: "npm test",
		Extensions: []string{".js", ".jsx", ".mjs"},
	},
	"typescript": {
		Linter:     "eslint",
		Formatter:  "prettier",
		Compiler:   "tsc --noEmit",
		TestRunner: "npm test",
		Extensions: []string{".ts", ".tsx"},
	},
	"python": {
		Linter:     "pylint",
		Formatter:  "black",
		Compiler:   "python -m py_compile",
		TestRunner: "pytest",
		Extensions: []string{".py"},
	},
	"rust": {
		Linter:     "clippy",
		Formatter:  "rustfmt",
		Compiler:   "cargo check",
		TestRunner: "cargo test",
		Extensions: []string{".rs"},
	},
	"ruby": {
		Linter:     "rubocop",
		Formatter:  "rubocop -a",
		Compiler:   "ruby -c",
		TestRunner: "rspec",
		Extensions: []string{".rb"},
	},
}

// CodeToolConfig holds tool configuration for a language
type CodeToolConfig struct {
	Linter     string
	Formatter  string
	Compiler   string
	TestRunner string
	Extensions []string
}

// CodeIssueSeverity levels
const (
	CodeSeverityError   = "error"
	CodeSeverityWarning = "warning"
	CodeSeverityInfo    = "info"
	CodeSeverityHint    = "hint"
)
