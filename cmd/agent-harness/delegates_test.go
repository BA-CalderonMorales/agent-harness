package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/state"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func TestHomeNewChatCreatesDistinctSession(t *testing.T) {
	sessionDir := t.TempDir()
	t.Setenv("AGENT_HARNESS_SESSION_DIR", sessionDir)

	sm, err := state.NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	app := &App{
		config:         &config.LayeredConfig{PermissionMode: config.PermissionWorkspaceWrite},
		sessionManager: sm,
		costTracker:    agent.NewCostTracker(),
	}
	app.costTracker.SetModel("test-model")
	app.session = sm.CreateSession("test-model")
	app.session.Persona = "developer"
	oldID := app.session.ID
	app.session.AddMessage(types.Message{
		UUID:      "msg-1",
		Role:      types.RoleUser,
		Timestamp: time.Now(),
		Content:   []types.ContentBlock{types.TextBlock{Text: "keep me"}},
	})
	if _, err := sm.SaveCurrent(); err != nil {
		t.Fatalf("SaveCurrent() error = %v", err)
	}

	tuiApp := tui.NewApp()
	delegate := &tuiHomeDelegate{app: app, tuiApp: tuiApp}
	delegate.OnNewChat()

	if app.session.ID == oldID {
		t.Fatalf("new chat reused old session ID %s", oldID)
	}
	if sm.GetCurrent().ID != app.session.ID {
		t.Fatalf("session manager current = %s, app session = %s", sm.GetCurrent().ID, app.session.ID)
	}

	oldSession, err := state.LoadSession(filepath.Join(sessionDir, oldID+".json"))
	if err != nil {
		t.Fatalf("old session was not retained: %v", err)
	}
	if len(oldSession.Messages) != 1 {
		t.Fatalf("old session message count = %d, want 1", len(oldSession.Messages))
	}

	if _, err := os.Stat(filepath.Join(sessionDir, app.session.ID+".json")); !os.IsNotExist(err) {
		t.Fatalf("new empty session should not eagerly overwrite/create a file, stat err = %v", err)
	}
}
