package main

import (
	"os"
	"path/filepath"
	"strings"
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

	oldSession, err := sm.ReadSession(oldID)
	if err != nil {
		t.Fatalf("old session was not retained: %v", err)
	}
	if len(oldSession.Messages) != 1 {
		t.Fatalf("old session message count = %d, want 1", len(oldSession.Messages))
	}

	if _, err := os.Stat(sm.GetDefaultSessionPath()); err != nil {
		t.Fatalf("new empty session should be persisted immediately, stat err = %v", err)
	}
}

func TestHomeExportSessionWritesCurrentSession(t *testing.T) {
	sessionDir := t.TempDir()
	exportDir := t.TempDir()
	t.Setenv("AGENT_HARNESS_SESSION_DIR", sessionDir)
	t.Setenv("HOME", t.TempDir())
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(exportDir); err != nil {
		t.Fatalf("Chdir(%q) error = %v", exportDir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalWD); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})

	sm, err := state.NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	app := &App{
		config:         &config.LayeredConfig{PermissionMode: config.PermissionWorkspaceWrite},
		sessionManager: sm,
		costTracker:    agent.NewCostTracker(),
	}
	app.session = sm.CreateSession("test-model")
	app.session.Persona = "developer"
	app.session.AddMessage(types.Message{
		UUID:      "msg-export",
		Role:      types.RoleUser,
		Timestamp: time.Now(),
		Content:   []types.ContentBlock{types.TextBlock{Text: "export me"}},
	})

	delegate := &tuiHomeDelegate{app: app, tuiApp: tui.NewApp()}
	delegate.OnExportSession()

	matches, err := filepath.Glob(filepath.Join(exportDir, "session-*.txt"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("export files = %d, want 1 (%v)", len(matches), matches)
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", matches[0], err)
	}
	if !strings.Contains(string(data), "export me") {
		t.Fatalf("export did not include session content: %s", string(data))
	}
}

func TestHomeLoadSessionKeepsCurrentOnMissingSession(t *testing.T) {
	sessionDir := t.TempDir()
	t.Setenv("AGENT_HARNESS_SESSION_DIR", sessionDir)

	sm, err := state.NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}
	current := sm.CreateSession("test-model")
	current.Persona = "developer"

	app := &App{
		config:         &config.LayeredConfig{PermissionMode: config.PermissionWorkspaceWrite},
		session:        current,
		sessionManager: sm,
		costTracker:    agent.NewCostTracker(),
	}

	delegate := &tuiHomeDelegate{app: app, tuiApp: tui.NewApp()}
	delegate.OnLoadSession("missing-session-id")

	if app.session.ID != current.ID {
		t.Fatalf("current session changed to %s, want %s", app.session.ID, current.ID)
	}
	if sm.GetCurrent().ID != current.ID {
		t.Fatalf("manager current session changed to %s, want %s", sm.GetCurrent().ID, current.ID)
	}
}

func TestHomeLoadSessionReplacesCurrentAndPreservesPersona(t *testing.T) {
	sessionDir := t.TempDir()
	t.Setenv("AGENT_HARNESS_SESSION_DIR", sessionDir)

	sm, err := state.NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}
	original := sm.CreateSession("old-model")
	loaded := sm.CreateSession("new-model")
	loaded.Persona = "reviewer"
	loaded.AddMessage(types.Message{
		UUID:      "msg-loaded",
		Role:      types.RoleAssistant,
		Timestamp: time.Now(),
		Content:   []types.ContentBlock{types.TextBlock{Text: "loaded content"}},
	})
	if _, err := sm.SaveCurrent(); err != nil {
		t.Fatalf("SaveCurrent() error = %v", err)
	}

	app := &App{
		config:         &config.LayeredConfig{PermissionMode: config.PermissionWorkspaceWrite},
		session:        original,
		sessionManager: sm,
		costTracker:    agent.NewCostTracker(),
	}

	delegate := &tuiHomeDelegate{app: app, tuiApp: tui.NewApp()}
	delegate.OnLoadSession(loaded.ID)

	if app.session.ID != loaded.ID {
		t.Fatalf("loaded session ID = %s, want %s", app.session.ID, loaded.ID)
	}
	if app.session.Persona != "reviewer" {
		t.Fatalf("loaded persona = %q, want reviewer", app.session.Persona)
	}
	if sm.GetCurrent().ID != loaded.ID {
		t.Fatalf("manager current session = %s, want %s", sm.GetCurrent().ID, loaded.ID)
	}
}

func TestSettingsPermissionModeChangeSyncsGranularToggles(t *testing.T) {
	app := &App{
		config:      &config.LayeredConfig{PermissionMode: config.PermissionWorkspaceWrite},
		session:     &state.Session{},
		costTracker: agent.NewCostTracker(),
	}
	delegate := &tuiSettingsDelegate{app: app, tuiApp: tui.NewApp()}

	delegate.handlePermissionModeChange(config.PermissionReadOnly.String())
	if app.config.PermissionMode != config.PermissionReadOnly {
		t.Fatalf("permission mode = %s, want %s", app.config.PermissionMode, config.PermissionReadOnly)
	}
	if !app.config.PermRead || app.config.PermWrite || app.config.PermDelete || app.config.PermExecute {
		t.Fatalf("read-only toggles = read:%v write:%v delete:%v execute:%v",
			app.config.PermRead, app.config.PermWrite, app.config.PermDelete, app.config.PermExecute)
	}

	delegate.handlePermissionModeChange(config.PermissionDangerFullAccess.String())
	if app.config.PermissionMode != config.PermissionDangerFullAccess {
		t.Fatalf("permission mode = %s, want %s", app.config.PermissionMode, config.PermissionDangerFullAccess)
	}
	if !app.config.PermRead || !app.config.PermWrite || !app.config.PermDelete || !app.config.PermExecute {
		t.Fatalf("danger-full-access toggles = read:%v write:%v delete:%v execute:%v",
			app.config.PermRead, app.config.PermWrite, app.config.PermDelete, app.config.PermExecute)
	}
}

func TestSettingsPermissionModeChangeIgnoresInvalidValue(t *testing.T) {
	app := &App{
		config: &config.LayeredConfig{
			PermissionMode: config.PermissionWorkspaceWrite,
			PermRead:       true,
			PermWrite:      true,
		},
		session:     &state.Session{},
		costTracker: agent.NewCostTracker(),
	}
	delegate := &tuiSettingsDelegate{app: app, tuiApp: tui.NewApp()}

	delegate.handlePermissionModeChange("not-a-mode")

	if app.config.PermissionMode != config.PermissionWorkspaceWrite {
		t.Fatalf("permission mode = %s, want %s", app.config.PermissionMode, config.PermissionWorkspaceWrite)
	}
	if !app.config.PermRead || !app.config.PermWrite || app.config.PermDelete || app.config.PermExecute {
		t.Fatalf("toggles changed after invalid mode: read:%v write:%v delete:%v execute:%v",
			app.config.PermRead, app.config.PermWrite, app.config.PermDelete, app.config.PermExecute)
	}
}

func TestBoolToEnabled(t *testing.T) {
	if got := boolToEnabled(true); got != "enabled" {
		t.Fatalf("boolToEnabled(true) = %q, want enabled", got)
	}
	if got := boolToEnabled(false); got != "disabled" {
		t.Fatalf("boolToEnabled(false) = %q, want disabled", got)
	}
}
