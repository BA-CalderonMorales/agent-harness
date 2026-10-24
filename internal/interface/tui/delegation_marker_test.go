package tui

import (
	"strings"
	"testing"
)

// A delegation is a flight between places. The user should see the
// sub-agent leave, not infer it from a spinner.

func TestWorkingLineShowsTheDelegation(t *testing.T) {
	var m ChatModel
	m.width = 80
	m.thinking = true
	m.currentTool = &ToolUseBlock{ID: "t1", Name: "agent"}

	line := m.workingStatusLine(0)
	if !strings.Contains(line, "✈") || !strings.Contains(line, "delegating") {
		t.Fatalf("working line = %q, want the delegation marker", line)
	}
}

func TestWorkingLineHasNoDelegationMarkerForOtherTools(t *testing.T) {
	var m ChatModel
	m.width = 80
	m.thinking = true
	m.currentTool = &ToolUseBlock{ID: "t1", Name: "bash"}

	line := m.workingStatusLine(0)
	if strings.Contains(line, "✈") {
		t.Fatalf("working line = %q, want no delegation marker for a shell call", line)
	}
}
