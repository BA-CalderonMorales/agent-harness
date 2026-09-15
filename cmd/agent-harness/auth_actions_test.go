package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/state"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// TestResetDeletesProjectSessions pins the portability fix: /reset must
// delete the sessions the SessionManager owns (AGENT_HARNESS_SESSION_DIR
// or the XDG data home), not a hand-rolled $HOME/.config path that is
// unset on Windows and names files the store never writes (.json vs the
// timestamped .jsonl per project directory).
func TestResetDeletesProjectSessions(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("AGENT_HARNESS_SESSION_DIR", filepath.Join(root, "sessions"))

	sm, err := state.NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	app := &App{sessionManager: sm}
	app.session = sm.CreateSession("test-model")
	sm.SetCurrent(app.session)
	app.session.AddMessage(types.Message{
		UUID:      "msg-1",
		Role:      types.RoleUser,
		Timestamp: time.Now(),
		Content:   []types.ContentBlock{types.TextBlock{Text: "keep me"}},
	})
	if _, err := sm.SaveCurrent(); err != nil {
		t.Fatalf("SaveCurrent() error = %v", err)
	}
	second := sm.CreateSession("test-model")
	sm.SetCurrent(second)
	if _, err := sm.SaveCurrent(); err != nil {
		t.Fatalf("SaveCurrent() second error = %v", err)
	}

	before, err := sm.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(before) != 2 {
		t.Fatalf("ListSessions() = %d sessions, want 2", len(before))
	}

	if err := app.reset(); err != nil {
		t.Fatalf("reset() error = %v", err)
	}

	after, err := sm.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions() after reset error = %v", err)
	}
	if len(after) != 0 {
		t.Fatalf("ListSessions() after reset = %d sessions, want 0", len(after))
	}
	if len(app.session.Messages) != 0 {
		t.Fatalf("reset() left %d messages in memory, want 0", len(app.session.Messages))
	}
	if sm.GetCurrent() == nil || sm.GetCurrent().ID != app.session.ID {
		t.Fatal("reset() must leave the cleared session as the manager current")
	}
}
