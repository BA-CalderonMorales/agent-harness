package defaults

// Mode defaults for permission transformations.
const (
	// DefaultMode is the default permission mode.
	DefaultMode = "default"

	// MaxConsecutiveDenials before auto-mode fallback.
	MaxConsecutiveDenials = 3
)

// ModeDescriptions explains each mode.
var ModeDescriptions = map[string]string{
	"default":           "Standard permission checking with user prompts",
	"acceptEdits":       "Automatically accept file edit operations",
	"dontAsk":           "Auto-allow non-destructive operations",
	"plan":              "Require approval for all operations",
	"auto":              "Use classifier for automatic decisions",
	"bypassPermissions": "Bypass all permission checks (requires flag)",
	"bubble":            "Bubble up permission decisions to parent",
}

// IsDestructiveTool returns true for tools that modify state.
func IsDestructiveTool(toolName string) bool {
	destructive := map[string]bool{
		"write":  true,
		"edit":   true,
		"bash":   true,
		"shell":  true,
		"rm":     true,
		"delete": true,
		"move":   true,
		"mv":     true,
	}
	return destructive[toolName]
}

// IsReadOnlyTool returns true for tools that only read.
func IsReadOnlyTool(toolName string) bool {
	readOnly := map[string]bool{
		"read":         true,
		"glob":         true,
		"ls_recursive": true,
		"grep":         true,
		"search":       true,
		"webfetch":     true,
		"websearch":    true,
	}
	return readOnly[toolName]
}
