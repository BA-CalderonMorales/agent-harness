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

// emptyStateLines renders the panel and strips the centering padding.
func emptyStateLines(t *testing.T, m ChatModel) []string {
	t.Helper()
	view := m.View()
	var lines []string
	for _, line := range strings.Split(view, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

// TestChatEmptyStatePanel pins the panel a fresh pane renders: what to
// ask first (the persona hint), what the agent does, which keys move
// you — quoted keys, bullet lines, capped height.
func TestChatEmptyStatePanel(t *testing.T) {
	m := newEmptyChatTestModel()
	got := strings.Join(emptyStateLines(t, m), "\n")
	for _, want := range []string{
		"The agent is ready.",
		"Ask it to Describe a feature to build or a bug to fix.",
		`"i" to start · "/" commands · "?" help`,
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

	got := strings.Join(emptyStateLines(t, m), "\n")
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

// TestChatEmptyStateSurvivesSystemNotice pins the New-Chat flow: the
// "Started new chat" notice renders above the panel, and the panel's
// centering shrinks by the notice's rows — placed at full height the
// combined block overflows and MaxHeight clips the text out of view.
func TestChatEmptyStateSurvivesSystemNotice(t *testing.T) {
	m := newEmptyChatTestModel()
	m.AddMessage("system", "Started new chat 27059f53")
	m.refreshViewport()

	lines := emptyStateLines(t, m)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "Started new chat 27059f53") {
		t.Fatalf("notice missing:\n%s", joined)
	}
	if !strings.Contains(joined, "The agent is ready.") {
		t.Fatalf("panel clipped by the notice:\n%s", joined)
	}
	if !strings.Contains(joined, `"i" to start · "/" commands · "?" help`) {
		t.Fatalf("panel key line clipped:\n%s", joined)
	}
}
