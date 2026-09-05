package tui

import (
	"strings"
	"testing"
)

func newEmptyChatTestModel() ChatModel {
	m := NewChatModel()
	m.width = 120
	m.viewport.Width = 120
	m.viewport.Height = 20
	m.height = 40
	return m
}

// TestChatEmptyStatePanel pins the panel a fresh pane renders: what to
// ask first (the persona hint), what the agent does, which keys move
// you — quoted keys, bullet lines, capped height.
func TestChatEmptyStatePanel(t *testing.T) {
	m := newEmptyChatTestModel()
	got := chatEmptyState(m.persona)
	for _, want := range []string{
		"The agent is ready.",
		`"i" to start`,
		`"/" for commands`,
		`"?" for the full map`,
		`"h" jumps Home`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("empty-state panel missing %q:\n%s", want, got)
		}
	}
	lines := strings.Count(got, "\n") + 1
	if lines > 8 {
		t.Fatalf("empty-state panel grew to %d lines (cap 8)", lines)
	}
}

// TestChatEmptyStateFollowsPersona pins the persona seed: the panel's
// first ask mirrors the active persona's hint.
func TestChatEmptyStateFollowsPersona(t *testing.T) {
	m := newEmptyChatTestModel()
	m.persona = "developer"

	got := chatEmptyState(m.persona)
	if !strings.Contains(got, "Describe a feature to build or a bug to fix") {
		t.Fatalf("panel did not carry the developer hint:\n%s", got)
	}
}

// TestChatEmptyViewRendersPanel ties the panel into the real View: an
// empty transcript shows it, and the old internal chunk counter stays
// out of the assistant header.
func TestChatEmptyViewRendersPanel(t *testing.T) {
	m := newEmptyChatTestModel()
	view := m.View()
	if !strings.Contains(view, "The agent is ready.") {
		t.Fatalf("empty chat View did not render the panel:\n%s", view)
	}
	if strings.Contains(view, "chunks") {
		t.Fatalf("internal chunk counter leaked into the view:\n%s", view)
	}
}
