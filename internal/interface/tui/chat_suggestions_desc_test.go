package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// TestSuggestionsRenderDescriptions pins the dropdown contract: the
// registry descriptions ride right of each command.
func TestSuggestionsRenderDescriptions(t *testing.T) {
	m := newEmptyChatTestModel()
	m.SetCommandCompletions([]string{"/mode", "/modes"})
	m.SetCommandDescriptions(map[string]string{
		"/mode":  "Set or cycle the agent mode (manual/auto/plan/chat)",
		"/modes": "List agent modes, marking the active one",
	})
	m.showSuggestions = true
	m.suggestions = []string{"/mode", "/modes"}

	out := m.renderSuggestions()
	if !strings.Contains(out, "Set or cycle the agent mode") {
		t.Fatalf("suggestion missing description:\n%s", out)
	}
}

// TestSuggestionDescriptionTruncatedToWidth pins the width guard: a
// long description is clipped to the terminal, never wrapped or
// shoved off the edge.
func TestSuggestionDescriptionTruncatedToWidth(t *testing.T) {
	m := newEmptyChatTestModel()
	m.width = 40
	long := "a very long description that definitely overflows the forty column terminal we are testing with"
	m.SetCommandCompletions([]string{"/x"})
	m.SetCommandDescriptions(map[string]string{"/x": long})
	m.showSuggestions = true
	m.suggestions = []string{"/x"}

	out := m.renderSuggestions()
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "/x") && lipgloss.Width(line) > 40 {
			t.Fatalf("suggestion line overflows width 40 (%d cols): %q", lipgloss.Width(line), line)
		}
	}
	if strings.Contains(out, long) {
		t.Fatalf("description not truncated:\n%s", out)
	}
}
