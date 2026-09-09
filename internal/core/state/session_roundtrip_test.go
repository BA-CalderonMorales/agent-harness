package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// TestSessionRoundTripPreservesInterleaving is the reproducible-sessions
// promise as a contract: a turn with a tool interruption — text, tool
// call, tool result, more text — must survive save → load with roles,
// block order, and tool-use identity intact. Everything downstream
// (resume, export, the TUI's segmented rendering) stands on this.
func TestSessionRoundTripPreservesInterleaving(t *testing.T) {
	sessionsDir := t.TempDir()
	t.Setenv("AGENT_HARNESS_SESSION_DIR", sessionsDir)

	manager, err := NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	session := manager.CreateSession("test-model")
	session.ID = "roundtrip-session"
	session.Persona = "scientist"
	session.PlanMode = true
	session.ToolLimit = 37
	session.AddMessage(types.Message{
		Role:    types.RoleUser,
		Content: []types.ContentBlock{types.TextBlock{Text: "weather?"}},
	})
	session.AddMessage(types.Message{
		Role: types.RoleAssistant,
		Content: []types.ContentBlock{
			types.TextBlock{Text: "On it — pulling the forecast."},
			types.ToolUseBlock{ID: "call-1", Name: "web_fetch", Input: map[string]any{"url": "https://wttr.in/Omaha"}},
			types.TextBlock{Text: "Got the data — tomorrow is a scorcher."},
		},
	})
	session.AddMessage(types.Message{
		Role:    types.RoleUser,
		Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: "call-1", Content: strings.Repeat("tool-output-", 10000)}},
	})

	if _, err := manager.SaveCurrent(); err != nil {
		t.Fatalf("SaveCurrent() error = %v", err)
	}

	// A fresh manager reads the same directory: resume, not memory.
	reloaded, err := NewSessionManager()
	if err != nil {
		t.Fatalf("reloaded manager error = %v", err)
	}
	loaded, err := reloaded.LoadSession("roundtrip-session")
	if err != nil {
		t.Fatalf("LoadSession() error = %v", err)
	}

	if len(loaded.Messages) != 3 {
		t.Fatalf("loaded %d messages, want 3", len(loaded.Messages))
	}

	assistant := loaded.Messages[1]
	if assistant.Role != types.RoleAssistant {
		t.Fatalf("assistant role = %q", assistant.Role)
	}
	if len(assistant.Content) != 3 {
		t.Fatalf("assistant blocks = %d, want 3 (text, tool_use, text) — the interleaving is the contract", len(assistant.Content))
	}
	if text, ok := assistant.Content[0].(types.TextBlock); !ok || text.Text != "On it — pulling the forecast." {
		t.Fatalf("block 0 = %#v, want the opening prose", assistant.Content[0])
	}
	if use, ok := assistant.Content[1].(types.ToolUseBlock); !ok || use.ID != "call-1" || use.Name != "web_fetch" {
		t.Fatalf("block 1 = %#v, want the web_fetch tool use with its ID", assistant.Content[1])
	}
	if result, ok := loaded.Messages[2].Content[0].(types.ToolResultBlock); !ok || result.ToolUseID != "call-1" {
		t.Fatalf("result block = %#v, want the tool result bound to call-1", loaded.Messages[2].Content[0])
	}
	if result := loaded.Messages[2].Content[0].(types.ToolResultBlock); len(result.Content) != len(strings.Repeat("tool-output-", 10000)) {
		t.Fatalf("large tool result length = %d, want preserved output", len(result.Content))
	}
	if loaded.Model != session.Model || loaded.Persona != session.Persona || !loaded.PlanMode || loaded.ToolLimit != session.ToolLimit {
		t.Fatalf("loaded settings = model %q, persona %q, plan %v, tool limit %d; want model %q, persona %q, plan %v, tool limit %d",
			loaded.Model, loaded.Persona, loaded.PlanMode, loaded.ToolLimit,
			session.Model, session.Persona, session.PlanMode, session.ToolLimit)
	}
}

func TestSessionManagerResumeUsesMostRecentlyWrittenSession(t *testing.T) {
	sessionsDir := t.TempDir()
	manager, err := NewSessionManagerWithDir(sessionsDir)
	if err != nil {
		t.Fatalf("NewSessionManagerWithDir() error = %v", err)
	}

	old := manager.CreateSession("old-model")
	old.ID = "old-session"
	old.CreatedAt = time.Now().Add(-2 * time.Hour)
	old.AddMessage(types.Message{Role: types.RoleUser, Content: []types.ContentBlock{types.TextBlock{Text: "old"}}})
	oldPath, err := manager.SaveCurrent()
	if err != nil {
		t.Fatalf("save old session: %v", err)
	}

	newer := manager.CreateSession("new-model")
	newer.ID = "new-session"
	newer.CreatedAt = time.Now().Add(-time.Hour)
	newer.AddMessage(types.Message{Role: types.RoleUser, Content: []types.ContentBlock{types.TextBlock{Text: "new"}}})
	newPath, err := manager.SaveCurrent()
	if err != nil {
		t.Fatalf("save newer session: %v", err)
	}
	now := time.Now()
	if err := os.Chtimes(newPath, now.Add(-time.Minute), now.Add(-time.Minute)); err != nil {
		t.Fatalf("touch newer session: %v", err)
	}
	if err := os.Chtimes(oldPath, now, now); err != nil {
		t.Fatalf("touch old session: %v", err)
	}

	fresh, err := NewSessionManagerWithDir(sessionsDir)
	if err != nil {
		t.Fatalf("fresh manager: %v", err)
	}
	resumed, ok := fresh.ResumeLatestSession()
	if !ok || resumed.ID != "old-session" {
		t.Fatalf("resumed = %#v, %v; want old-session, true", resumed, ok)
	}
}

func TestSessionManagerKeepsProjectSessionsScoped(t *testing.T) {
	sessionsDir := t.TempDir()
	t.Setenv("AGENT_HARNESS_SESSION_DIR", sessionsDir)
	projectA := filepath.Join(t.TempDir(), "project-a")
	projectB := filepath.Join(t.TempDir(), "project-b")
	managerA, err := NewSessionManagerForProject(projectA)
	if err != nil {
		t.Fatalf("manager A: %v", err)
	}
	managerB, err := NewSessionManagerForProject(projectB)
	if err != nil {
		t.Fatalf("manager B: %v", err)
	}
	// Both managers use the same disposable root, while their project slugs
	// select independent durable session scopes.
	session := managerA.CreateSession("model")
	session.ID = "project-a-session"
	if _, err := managerA.SaveCurrent(); err != nil {
		t.Fatalf("save project A: %v", err)
	}
	if _, err := managerB.LoadSession(session.ID); err == nil {
		t.Fatal("project B loaded project A session")
	}
}

func TestSessionManagerRejectsTruncatedPersistence(t *testing.T) {
	sessionsDir := t.TempDir()
	manager, err := NewSessionManagerWithDir(sessionsDir)
	if err != nil {
		t.Fatalf("NewSessionManagerWithDir() error = %v", err)
	}
	session := manager.CreateSession("model")
	session.ID = "truncated-session"
	session.AddMessage(types.Message{Role: types.RoleUser, Content: []types.ContentBlock{types.TextBlock{Text: strings.Repeat("large-record ", 1000)}}})
	path, err := manager.SaveCurrent()
	if err != nil {
		t.Fatalf("SaveCurrent() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read session: %v", err)
	}
	if err := os.WriteFile(path, append(data, []byte(`{"type":"message","message":`)...), 0600); err != nil {
		t.Fatalf("truncate session: %v", err)
	}

	fresh, err := NewSessionManagerWithDir(sessionsDir)
	if err != nil {
		t.Fatalf("fresh manager: %v", err)
	}
	if _, err := fresh.LoadSession(session.ID); err == nil || !strings.Contains(err.Error(), "corrupt") {
		t.Fatalf("LoadSession() error = %v, want explicit corruption error", err)
	}
	if resumed, ok := fresh.ResumeLatestSession(); ok || resumed != nil {
		t.Fatalf("ResumeLatestSession() = %#v, %v; must not claim damaged data recovered", resumed, ok)
	}
	if resumed, err := fresh.ResumeLatestSessionWithError(); resumed != nil || err == nil || !strings.Contains(err.Error(), "could not be resumed") {
		t.Fatalf("ResumeLatestSessionWithError() = %#v, %v; want safe recovery error", resumed, err)
	}
}

func TestSessionManagerWithDirMigratesLegacySession(t *testing.T) {
	sessionsDir := t.TempDir()
	legacy := NewSession("legacy-model")
	legacy.ID = "legacy-session"
	legacy.Persona = "scientist"
	legacy.PlanMode = true
	legacy.ToolLimit = 19
	legacy.AddMessage(types.Message{Role: types.RoleUser, Content: []types.ContentBlock{
		types.TextBlock{Text: "legacy history"},
	}})
	legacyPath := filepath.Join(sessionsDir, legacy.ID+".json")
	if err := legacy.SaveToFile(legacyPath); err != nil {
		t.Fatalf("write legacy session: %v", err)
	}

	manager, err := NewSessionManagerWithDir(sessionsDir)
	if err != nil {
		t.Fatalf("NewSessionManagerWithDir() error = %v", err)
	}
	loaded, err := manager.LoadSession(legacy.ID)
	if err != nil {
		t.Fatalf("LoadSession() error = %v", err)
	}
	if loaded.Model != legacy.Model || loaded.Persona != legacy.Persona || !loaded.PlanMode || loaded.ToolLimit != legacy.ToolLimit || len(loaded.Messages) != 1 {
		t.Fatalf("loaded legacy session = %#v, want settings and one message preserved", loaded)
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("legacy file stat error = %v, want migration to remove verified source", err)
	}
}

func TestSessionManagerAppendsAfterSwitchingSessions(t *testing.T) {
	sessionsDir := t.TempDir()
	manager, err := NewSessionManagerWithDir(sessionsDir)
	if err != nil {
		t.Fatalf("NewSessionManagerWithDir() error = %v", err)
	}

	first := manager.CreateSession("model")
	first.ID = "first-session"
	first.AddMessage(types.Message{Role: types.RoleUser, Content: []types.ContentBlock{types.TextBlock{Text: "one"}}})
	if _, err := manager.SaveCurrent(); err != nil {
		t.Fatalf("save first session: %v", err)
	}
	second := manager.CreateSession("model")
	second.ID = "second-session"
	if _, err := manager.SaveCurrent(); err != nil {
		t.Fatalf("save second session: %v", err)
	}

	loaded, err := manager.ReadSession(first.ID)
	if err != nil {
		t.Fatalf("read first session: %v", err)
	}
	loaded.AddMessage(types.Message{Role: types.RoleAssistant, Content: []types.ContentBlock{types.TextBlock{Text: "two"}}})
	manager.SetCurrent(loaded)
	if _, err := manager.SaveCurrent(); err != nil {
		t.Fatalf("save switched session: %v", err)
	}

	fresh, err := NewSessionManagerWithDir(sessionsDir)
	if err != nil {
		t.Fatalf("fresh manager: %v", err)
	}
	got, err := fresh.LoadSession(first.ID)
	if err != nil {
		t.Fatalf("reload switched session: %v", err)
	}
	if len(got.Messages) != 2 || got.Messages[0].Content[0].(types.TextBlock).Text != "one" || got.Messages[1].Content[0].(types.TextBlock).Text != "two" {
		t.Fatalf("messages after switch = %#v, want [one two]", got.Messages)
	}
}
