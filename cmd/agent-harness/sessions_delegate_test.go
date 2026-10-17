package main

import (
	"os"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/state"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
)

// Deleting the active session used to request a jump to Chat, so confirming
// a delete on Home sent the user to another tab — the feedback for an action
// taken on Home belonged somewhere else. The deleted session's replacement
// must still reach the Chat view, but without changing the active tab: the
// SessionActivatedMsg is sent with SwitchToChat false, which the TUI honors
// (app_ginkgo_test.go pins the false case).
func TestDeleteActiveSessionKeepsCurrentTab(t *testing.T) {
	t.Setenv("AGENT_HARNESS_SESSION_DIR", t.TempDir())

	sm, err := state.NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}
	active := sm.CreateSession("test-model")
	active.ID = "active-session"
	if _, err := sm.SaveCurrent(); err != nil {
		t.Fatalf("SaveCurrent() error = %v", err)
	}

	app := &App{
		config:         &config.LayeredConfig{PermissionMode: config.PermissionWorkspaceWrite},
		session:        active,
		sessionManager: sm,
		costTracker:    agent.NewCostTracker(),
	}
	tuiApp := tui.NewApp()

	app.deleteSession(active.ID, tuiApp)

	msg := receiveTUIMessage(t, tuiApp)
	activated, ok := msg.(tui.SessionActivatedMsg)
	if !ok {
		t.Fatalf("message = %T, want SessionActivatedMsg", msg)
	}
	if activated.SwitchToChat {
		t.Fatal("deleting the active session requested a switch to Chat; the user must stay on the tab they deleted from")
	}
	if activated.SessionID != app.session.ID {
		t.Fatalf("activated session = %q, want the replacement %q", activated.SessionID, app.session.ID)
	}
	if app.session.ID == active.ID {
		t.Fatalf("active session id = %q, want a replacement", app.session.ID)
	}
	if _, err := sm.ReadSession(active.ID); err == nil {
		t.Fatalf("deleted session %q is still readable", active.ID)
	}
}

// Deleting a session that is not active already left the tab alone; it must
// keep refreshing the session lists without activating anything.
func TestDeleteInactiveSessionRefreshesWithoutActivating(t *testing.T) {
	t.Setenv("AGENT_HARNESS_SESSION_DIR", t.TempDir())

	sm, err := state.NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}
	active := sm.CreateSession("test-model")
	active.ID = "active-session"
	if _, err := sm.SaveCurrent(); err != nil {
		t.Fatalf("SaveCurrent(active) error = %v", err)
	}
	gone := sm.CreateSession("test-model")
	gone.ID = "gone-session"
	if _, err := sm.SaveCurrent(); err != nil {
		t.Fatalf("SaveCurrent(gone) error = %v", err)
	}
	sm.SetCurrent(active)

	app := &App{
		config:         &config.LayeredConfig{PermissionMode: config.PermissionWorkspaceWrite},
		session:        active,
		sessionManager: sm,
		costTracker:    agent.NewCostTracker(),
	}
	tuiApp := tui.NewApp()

	app.deleteSession(gone.ID, tuiApp)

	msg := receiveTUIMessage(t, tuiApp)
	if _, ok := msg.(tui.SessionsRefreshedMsg); !ok {
		t.Fatalf("message = %T, want SessionsRefreshedMsg", msg)
	}
	if app.session.ID != active.ID {
		t.Fatalf("active session = %q, want it unchanged at %q", app.session.ID, active.ID)
	}
	if _, err := os.Stat(sm.GetDefaultSessionPath()); err != nil {
		t.Fatalf("active session file missing: %v", err)
	}
	if _, err := sm.ReadSession(gone.ID); err == nil {
		t.Fatalf("deleted session %q is still readable", gone.ID)
	}
}
