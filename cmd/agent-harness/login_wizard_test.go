package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// TestWizardModelsLiveList is the lightning bolt: the wizard's model
// step shows the actual models the candidate endpoint serves, with the
// provider's pinned default marked.
func TestWizardModelsLiveList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/models") {
			t.Fatalf("probe hit %s, want /models", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data": []map[string]any{
				{"id": "gpt-4o", "object": "model"},
				{"id": "demo-1.0", "object": "model"},
			},
		})
	}))
	defer server.Close()

	app := &App{config: &config.LayeredConfig{EndpointURL: server.URL, EndpointPinned: true}}
	items, err := app.wizardModels("openai", "sk-test")
	if err != nil {
		t.Fatalf("wizardModels error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("wizardModels returned %d items, want the 2 live models", len(items))
	}
	ids := map[string]bool{}
	pins := 0
	for _, item := range items {
		ids[item.ID] = true
		if item.IsDefault {
			pins++
			if item.ID != getDefaultModel("openai") {
				t.Fatalf("IsDefault marked on %q, want the pinned default %q", item.ID, getDefaultModel("openai"))
			}
		}
	}
	if !ids["gpt-4o"] || !ids["demo-1.0"] {
		t.Fatalf("live model ids = %v, want gpt-4o and demo-1.0", ids)
	}
	if pins != 1 {
		t.Fatalf("IsDefault pins = %d, want exactly 1 (%q)", pins, getDefaultModel("openai"))
	}
}

// TestWizardModelsFailingProbeFallsBackToCatalog: a dead endpoint must
// surface an honest error next to the static catalog, never a silent
// default.
func TestWizardModelsFailingProbeFallsBackToCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer server.Close()

	app := &App{config: &config.LayeredConfig{EndpointURL: server.URL, EndpointPinned: true}}
	items, err := app.wizardModels("openai", "sk-test")
	if err == nil {
		t.Fatal("failing probe must return an error")
	}
	if len(items) == 0 {
		t.Fatal("failing probe must still return the static catalog")
	}
	if items[0].Provider != "openai" {
		t.Fatalf("catalog items provider = %q, want openai", items[0].Provider)
	}
}

// TestWizardEndpointPinning: an env-pinned endpoint survives the login;
// otherwise the provider default applies - the probe and completeLogin
// must agree on the endpoint or the green check would lie.
func TestWizardEndpointPinning(t *testing.T) {
	app := &App{config: &config.LayeredConfig{EndpointURL: "http://127.0.0.1:9999/v1", EndpointPinned: true}}
	if got := app.wizardEndpoint("openai"); got != "http://127.0.0.1:9999/v1" {
		t.Fatalf("pinned endpoint = %q, want the env-pinned URL", got)
	}

	app = &App{config: &config.LayeredConfig{}}
	if got := app.wizardEndpoint("openai"); got != config.DefaultEndpointForProvider("openai") {
		t.Fatalf("unpinned endpoint = %q, want the provider default", got)
	}
}

// TestStartFreshSessionPersistsPrevious pins the rotation contract: the
// current session is saved first (its history stays resumable), the
// persona carries over, and the manager re-anchors on the fresh session.
func TestStartFreshSessionPersistsPrevious(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("AGENT_HARNESS_SESSION_DIR", filepath.Join(root, "sessions"))

	sm, err := state.NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	app := &App{
		config:         &config.LayeredConfig{PermissionMode: config.PermissionWorkspaceWrite},
		sessionManager: sm,
		costTracker:    agent.NewCostTracker(),
	}
	app.session = sm.CreateSession("old-model")
	app.session.Persona = "developer"
	app.session.AddMessage(types.Message{
		UUID:      "msg-1",
		Role:      types.RoleUser,
		Timestamp: time.Now(),
		Content:   []types.ContentBlock{types.TextBlock{Text: "keep me"}},
	})
	if _, err := sm.SaveCurrent(); err != nil {
		t.Fatalf("SaveCurrent() error = %v", err)
	}
	oldID := app.session.ID

	if err := app.startFreshSession("new-model"); err != nil {
		t.Fatalf("startFreshSession() error = %v", err)
	}

	if app.session.ID == oldID {
		t.Fatal("startFreshSession reused the old session ID")
	}
	if len(app.session.Messages) != 0 {
		t.Fatalf("fresh session carries %d messages, want 0", len(app.session.Messages))
	}
	if app.session.Model != "new-model" {
		t.Fatalf("fresh session model = %q, want new-model", app.session.Model)
	}
	if app.session.Persona != "developer" {
		t.Fatalf("fresh session persona = %q, want the persona carried over", app.session.Persona)
	}
	if sm.GetCurrent().ID != app.session.ID {
		t.Fatal("manager current did not re-anchor on the fresh session")
	}
	old, err := sm.ReadSession(oldID)
	if err != nil {
		t.Fatalf("previous session was not retained: %v", err)
	}
	if len(old.Messages) != 1 {
		t.Fatalf("previous session has %d messages, want 1", len(old.Messages))
	}
}

// TestCompleteLoginGreetsWithFreshSession pins the authenticated happy
// path: a successful login rotates to a fresh session (the previous one
// retained), and the greeting points at /help.
func TestCompleteLoginGreetsWithFreshSession(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("AGENT_HARNESS_SESSION_DIR", filepath.Join(root, "sessions"))

	sm, err := state.NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	app := &App{
		config:         &config.LayeredConfig{PermissionMode: config.PermissionWorkspaceWrite},
		sessionManager: sm,
		costTracker:    agent.NewCostTracker(),
		cwd:            root,
	}
	app.session = sm.CreateSession("old-model")
	app.session.AddMessage(types.Message{
		UUID:      "msg-1",
		Role:      types.RoleUser,
		Timestamp: time.Now(),
		Content:   []types.ContentBlock{types.TextBlock{Text: "prior work"}},
	})
	if _, err := sm.SaveCurrent(); err != nil {
		t.Fatalf("SaveCurrent() error = %v", err)
	}
	oldID := app.session.ID

	app.completeLogin("local", "", "local-model", tui.NewApp())

	if app.session == nil || app.session.ID == oldID {
		t.Fatal("completeLogin must greet with a fresh session, not the old one")
	}
	if len(app.session.Messages) != 0 {
		t.Fatalf("greeting session carries %d messages, want 0", len(app.session.Messages))
	}
	if app.session.Model != "local-model" {
		t.Fatalf("greeting session model = %q, want local-model", app.session.Model)
	}
	if sm.GetCurrent().ID != app.session.ID {
		t.Fatal("manager current must anchor on the greeting session")
	}
	old, err := sm.ReadSession(oldID)
	if err != nil {
		t.Fatalf("previous session was not retained by login: %v", err)
	}
	if len(old.Messages) != 1 {
		t.Fatalf("previous session has %d messages, want 1", len(old.Messages))
	}
}
