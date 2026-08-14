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
