package tui

import (
	"strings"
	"testing"
)

// The home base strip draws real state: the workspace, the branch, the
// provider, and the sessions. It must never invent any of it.

func TestHomeBaseStripDrawsRealState(t *testing.T) {
	var m HomeModel
	m.width = 80
	m.SetProjectInfo(ProjectInfo{Name: "agent-harness", Type: "Go", GitBranch: "release/0.4.0", HasChanges: true})
	m.SetProvider("openrouter")
	m.SetStatus("gpt-5.1", "interactive", "builder", 4200)
	m.SetSessions([]SessionInfo{
		{ID: "1", Title: "one", UnreadableCount: 2},
		{ID: "2", Title: "two"},
	})

	strip := m.homeBaseStrip(80)
	for _, want := range []string{"⌂", "agent-harness", "release/0.4.0*", "Go", "openrouter", "2 sessions"} {
		if !strings.Contains(strip, want) {
			t.Fatalf("strip %q is missing %q", strip, want)
		}
	}
}

func TestHomeBaseStripIsAlwaysTwoLines(t *testing.T) {
	var m HomeModel
	m.width = 80
	strip := m.homeBaseStrip(80)
	if lines := strings.Count(strip, "\n") + 1; lines != 2 {
		t.Fatalf("strip has %d lines, want 2 so the pinned chrome keeps a fixed height:\n%s", lines, strip)
	}
}

func TestHomeBaseStripCollapsesOnANarrowPane(t *testing.T) {
	var m HomeModel
	m.width = 30
	m.SetProjectInfo(ProjectInfo{Name: "agent-harness", Type: "Go", GitBranch: "main"})
	m.SetProvider("openrouter")

	strip := m.homeBaseStrip(30)
	if strings.Contains(strip, "openrouter") {
		t.Fatalf("strip %q should drop the provider on a narrow pane", strip)
	}
	if !strings.Contains(strip, "agent-harness") {
		t.Fatalf("strip %q should keep the workspace", strip)
	}
}

func TestHomeStripIsDroppedOnAShortPane(t *testing.T) {
	if homeStripRows(12) == 0 {
		t.Fatal("the strip should stay on a pane of 12 rows")
	}
	if homeStripRows(8) != 0 {
		t.Fatal("the strip should be dropped on a pane of 8 rows")
	}
}
