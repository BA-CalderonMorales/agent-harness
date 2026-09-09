package tui

import (
	"strings"
	"testing"
	"time"
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

func TestHomeSelectedSessionStaysVisible(t *testing.T) {
	m := NewHomeModel()
	m.Init()
	m.SetSessions([]SessionInfo{{ID: "a", Title: "First"}, {ID: "b", Title: "Second"}, {ID: "c", Title: "Third"}, {ID: "d", Title: "Fourth"}})
	m.GotoBottom()
	if !strings.Contains(m.renderRecentSessions(), "Fourth") {
		t.Fatal("selected session below the preview is invisible")
	}
}
