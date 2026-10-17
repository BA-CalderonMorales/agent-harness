package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHomeRecentSessionsNewestFirst(t *testing.T) {
	now := time.Now()
	input := []SessionInfo{
		{ID: "old", Title: "Old", UpdatedAt: now.Add(-time.Hour)},
		{ID: "new", Title: "New", UpdatedAt: now},
		{ID: "middle", Title: "Middle", UpdatedAt: now.Add(-time.Minute)},
	}
	m := NewHomeModel()
	m.SetSessions(input)
	if m.sessions[0].ID != "new" || m.sessions[1].ID != "middle" {
		t.Fatalf("sessions not newest first: %+v", m.sessions)
	}
	if input[0].ID != "old" {
		t.Fatal("caller list mutated")
	}
}

// TestHomeSelectedSessionStaysVisible pins the scroll-follow on a pane too
// short to show the dashboard at once. Reaching the last session has to
// bring it into view: the dashboard used to render as one block clipped by
// the pane's height with no scroll window, so on a 40-row-tall pane a
// 100x20 terminal showed the "Quick Actions" heading and nothing else, and
// no session could be seen — or deleted.
func TestHomeSelectedSessionStaysVisible(t *testing.T) {
	m := NewHomeModel()
	m.Init()
	m.SetSessions([]SessionInfo{{ID: "a", Title: "First"}, {ID: "b", Title: "Second"}, {ID: "c", Title: "Third"}, {ID: "d", Title: "Fourth"}})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 14})

	m.GotoBottom()
	view := SanitizeANSI(m.View())
	if !strings.Contains(view, "Fourth") {
		t.Fatalf("selected session below the fold is invisible:\n%s", view)
	}
}

// TestHomeEveryRowIsReachable pins the fix for the reported problem: with
// more sessions than the pane can show, walking the cursor must reveal each
// one in turn, and the view must never exceed the pane it was given.
func TestHomeEveryRowIsReachable(t *testing.T) {
	sessions := make([]SessionInfo, 8)
	for i := range sessions {
		sessions[i] = SessionInfo{
			ID:        fmt.Sprintf("s%d", i),
			Title:     fmt.Sprintf("recent-%d", i),
			UpdatedAt: time.Now().Add(-time.Duration(i) * time.Minute),
		}
	}

	for _, height := range []int{24, 20, 16, 12, 8} {
		m := NewHomeModel()
		m.Init()
		m.SetSessions(sessions)
		m.Update(tea.WindowSizeMsg{Width: 100, Height: height})

		for i := 0; i < m.totalItems(); i++ {
			view := SanitizeANSI(m.View())
			if rows := strings.Count(view, "\n") + 1; rows > height {
				t.Fatalf("height %d: home rendered %d rows\n%s", height, rows, view)
			}
			if m.cursorInActions() {
				if !strings.Contains(view, m.actions[m.actionCursor].Label) {
					t.Fatalf("height %d: focused action %q not visible\n%s",
						height, m.actions[m.actionCursor].Label, view)
				}
			} else if want := sessions[m.cursorSessionIndex()].Title; !strings.Contains(view, want) {
				t.Fatalf("height %d: focused session %q not visible\n%s", height, want, view)
			}
			m.Scroll(1)
		}
	}
}
