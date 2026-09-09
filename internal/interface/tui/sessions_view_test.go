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
