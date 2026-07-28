package builtin

import "testing"

func TestBashToolUnclassifiedInputDefaultsToSerialAndDestructive(t *testing.T) {
	input := map[string]any{"command": "custom-unclassified-command --flag"}

	if BashTool.Capabilities.IsConcurrencySafe(input) {
		t.Error("unclassified Bash input was marked concurrency-safe; Bash must default to serial execution")
	}
	if !BashTool.Capabilities.IsDestructive(input) {
		t.Error("unclassified Bash input was marked non-destructive; Bash must default to destructive")
	}
}
