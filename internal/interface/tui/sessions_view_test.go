package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestSessionsFooterWrappedFitsPane(t *testing.T) {
	actions := []ActionHint{
		{Key: "↑/↓", Desc: "Navigate"},
		{Key: "Enter", Desc: "Select"},
		{Key: "n", Desc: "New"},
		{Key: "d", Desc: "Delete"},
		{Key: "e", Desc: "Export"},
		{Key: "c", Desc: "Copy"},
		{Key: "r", Desc: "Refresh"},
	}
	rendered := strings.TrimSuffix(RenderCompactFooterWrapped(actions, 36), "\n")
	lines := strings.Split(rendered, "\n")
	if len(lines) < 2 {
		t.Fatalf("footer should wrap to multiple lines, got %d: %q", len(lines), rendered)
	}
	for _, line := range lines {
		plain := strings.TrimPrefix(line, "  ")
		if w := lipgloss.Width(plain); w > 36 {
			t.Fatalf("footer line overflows pane: %q (%d > 36)", plain, w)
		}
	}
	for _, want := range []string{"Delete", "Export", "Refresh"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("footer lost %q: %q", want, rendered)
		}
	}
}

func TestSessionsNoticeLifecycle(t *testing.T) {
	m := NewSessionsModel()
	m.Focus()
	m.SetSessions([]SessionInfo{{ID: "abc123", Title: "Session abc123"}})
	m.SetNotice("Deleted session abc123", "success")
	if m.notice != "Deleted session abc123" || m.noticeType != "success" {
		t.Fatalf("notice not set: %q/%q", m.notice, m.noticeType)
	}

	// Navigation clears the notice.
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	updated := mm.(SessionsModel)
	if updated.notice != "" {
		t.Fatalf("notice survived navigation: %q", updated.notice)
	}
}

func TestSessionsWarningKeepsReadableRowsSelectable(t *testing.T) {
	m := NewSessionsModel()
	m.width = 80
	m.height = 20
	m.Focus()
	m.SetSessions([]SessionInfo{
		{ID: "good", Title: "Readable", UnreadableCount: 2},
		{UnreadableCount: 2},
	})
	if len(m.sessions) != 1 || m.sessions[0].ID != "good" {
		t.Fatalf("readable sessions = %#v, want only the readable row", m.sessions)
	}
	if !strings.Contains(m.View(), "2 saved sessions could not be") {
		t.Fatalf("warning missing from sessions view:\n%s", m.View())
	}
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if mm.(SessionsModel).cursor != 0 {
		t.Fatal("warning state changed the readable selection")
	}
}

func TestSessionsMobilePaneRendersWithoutPanic(t *testing.T) {
	for _, width := range []int{30, 35, 40, 50, 60, 80} {
		m := NewSessionsModel()
		m.width = width
		m.height = 24
		m.Focus()
		m.SetSessions([]SessionInfo{
			{ID: "session-1", Title: "Very long session title that will definitely need truncation on mobile", Turns: 2, MessageCount: 4},
			{ID: "session-2", Title: "Short", IsActive: true},
		})
		view := m.View()
		if view == "" {
			t.Fatalf("width %d rendered empty view", width)
		}
		if !strings.Contains(view, "All Sessions") {
			t.Fatalf("width %d missing title: %s", width, view)
		}
	}
}

// TestSessionRowFitsItsWidthBudget pins the row budget invariant: a
// session row is never rendered wider than the pane it was rendered for.
// The list styles pad two cells per side, the prefix leads the row, and
// the age and status badge trail it, so anything budgeted past the real
// remainder overruns the pane and the frame clip then cuts the trailing
// fields — the age and the status badge are exactly what tells two
// sessions apart. Widths start at 8 because the unstyled chrome needs six
// cells before a row can fit at all.
func TestSessionRowFitsItsWidthBudget(t *testing.T) {
	long := "Very long session title that will definitely need truncation on narrow panes"
	probes := []SessionInfo{
		{ID: "neutral", Title: long},
		{ID: "active", Title: long, IsActive: true},
		{ID: "short", Title: "Short"},
		{ID: "untitled", Title: ""},
	}
	m := NewSessionsModel()

	for _, probe := range probes {
		for _, selected := range []bool{false, true} {
			for width := 8; width <= 64; width++ {
				row := m.renderSessionItem(probe, selected, width)
				if got := lipgloss.Width(row); got > width {
					t.Errorf("%s selected=%v: width %d row is %d cells wide: %q",
						probe.ID, selected, width, got, strings.TrimSpace(row))
				}
			}
		}
	}
}

// TestSessionsNarrowHeightKeepsTheList covers the narrow-height layout
// path: the sessions tab must render its list title and its session rows
// at every phone height. Asserting the title alone would pass even if the
// pane rendered no rows at all, so the rows are asserted too.
//
// The content-height floor itself is inert at these sizes: the list
// always renders more rows than the floor reserves, so Height() never
// binds. Probing floor 5 against floor 8 produces identical output, so
// this test documents the rendered behaviour, not the floor's delta.
func TestSessionsNarrowHeightKeepsTheList(t *testing.T) {
	for _, height := range []int{8, 10, 12, 16, 24} {
		for _, width := range []int{30, 40, 60} {
			m := NewSessionsModel()
			m.width = width
			m.height = height
			m.Focus()
			m.SetSessions([]SessionInfo{
				{ID: "session-1", Title: "Very long session title that will need truncation"},
				{ID: "session-2", Title: "Short", IsActive: true},
			})
			view := m.View()
			if view == "" {
				t.Fatalf("w%d h%d rendered empty view", width, height)
			}
			if !strings.Contains(view, "All Sessions") {
				t.Fatalf("w%d h%d dropped the list title: %q", width, height, view)
			}
			if !strings.Contains(view, "Short") {
				t.Fatalf("w%d h%d rendered no session rows: %q", width, height, view)
			}
		}
	}
}
