package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// highlightedRows counts the rows that carry the selection indicator. The
// indicator is "> "; SanitizeANSI strips the styling, leaving the list
// style's two-cell left pad in front of it.
func highlightedRows(view string) int {
	count := 0
	for _, line := range strings.Split(view, "\n") {
		if strings.HasPrefix(strings.TrimLeft(line, " "), IndicatorSelected) {
			count++
		}
	}
	return count
}

// TestHomeHighlightsOnlyTheCursorRow pins the reported annoyance: the
// active session kept the selection highlight regardless of the cursor, so
// the Home tab showed "New chat" and the active session marked at the same
// time — two options that both looked ready to open. The highlight means
// "this is what Enter will act on", so exactly one row, the cursor's, may
// carry the indicator.
func TestHomeHighlightsOnlyTheCursorRow(t *testing.T) {
	m := NewHomeModel()
	m.Init()
	m.SetSessions([]SessionInfo{
		{ID: "a", Title: "First", IsActive: true},
		{ID: "b", Title: "Second"},
	})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Cursor parked on the first quick action: the active session must not
	// also be highlighted.
	view := SanitizeANSI(m.View())
	if got := highlightedRows(view); got != 1 {
		t.Fatalf("highlighted rows = %d, want exactly 1\n%s", got, view)
	}
	if !strings.Contains(view, IndicatorSelected+"New chat") {
		t.Fatalf("the cursor row is not the highlighted one:\n%s", view)
	}

	// Moving onto the active session leaves a single highlight, now on the
	// session.
	m.Scroll(len(m.actions))
	view = SanitizeANSI(m.View())
	if got := highlightedRows(view); got != 1 {
		t.Fatalf("highlighted rows after moving onto a session = %d, want 1\n%s", got, view)
	}
	if !strings.Contains(view, IndicatorSelected+"First") {
		t.Fatalf("active session is not the highlighted row:\n%s", view)
	}
}
