package tui

import (
	"strings"
	"testing"
)

func newGuidanceTestModel() ChatModel {
	m := NewChatModel()
	m.width = 120
	m.viewport.Width = 120
	m.viewport.Height = 20
	m.height = 40
	return m
}

// TestGuidanceShowsOncePerSession pins the once-only guard.
func TestGuidanceShowsOncePerSession(t *testing.T) {
	m := newGuidanceTestModel()
	m.ShowNavigationGuidance()
	first := len(m.messages)
	if first != 1 {
		t.Fatalf("first ShowNavigationGuidance added %d messages, want 1", first)
	}
	m.ShowNavigationGuidance()
	if len(m.messages) != first {
		t.Fatalf("second ShowNavigationGuidance added another message")
	}
	for _, msg := range m.messages {
		if msg.Role != "system" {
			t.Fatalf("guidance must be a system message, got %q", msg.Role)
		}
	}
}

// TestGuidanceRidesWithClear pins the /clear behavior: a wiped pane
// greets with guidance again.
func TestGuidanceRidesWithClear(t *testing.T) {
	m := newGuidanceTestModel()
	m.ShowNavigationGuidance()

	model, _ := m.Update(ClearChatMsg{})
	m = model.(ChatModel)
	joined := ""
	for _, msg := range m.messages {
		joined += msg.Content + "\n"
	}
	if !strings.Contains(joined, `"i" to start chatting`) {
		t.Fatalf("/clear did not re-show guidance:\n%s", joined)
	}

	// And the once-per-session guard is re-armed, not stuck on.
	count := len(m.messages)
	m.ShowNavigationGuidance()
	if len(m.messages) != count {
		t.Fatalf("guidance re-armed after /clear should not double-show")
	}
}

// TestGuidanceContentCoversCoreKeys keeps the block honest about the
// format and the keys it teaches: quoted keys, bullet lines, i, Esc,
// j/k, Enter, Shift+Tab, /.
func TestGuidanceContentCoversCoreKeys(t *testing.T) {
	got := navigationGuidance()
	for _, want := range []string{
		`• "i" to start chatting`,
		`• "Esc" to stop chatting`,
		`• "j" and "k" to scroll up and down`,
		`• "Enter" expands`,
		`• "Shift+Tab" cycles agent modes`,
		`"/" opens commands`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("guidance missing %q:\n%s", want, got)
		}
	}
	lines := strings.Count(got, "\n") + 1
	if lines > 6 {
		t.Fatalf("guidance grew to %d lines (cap 6)", lines)
	}
}
