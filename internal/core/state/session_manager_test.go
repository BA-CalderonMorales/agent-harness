package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func TestSessionManagerPersistsListsReadsAndResumesSessions(t *testing.T) {
	sessionsDir := t.TempDir()
	t.Setenv("AGENT_HARNESS_SESSION_DIR", sessionsDir)

	manager, err := NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	first := manager.CreateSession("first-model")
	first.ID = "first-session"
	first.AddMessage(types.Message{Role: types.RoleUser, Content: []types.ContentBlock{types.TextBlock{Text: "first"}}})
	firstPath, err := manager.SaveCurrent()
	if err != nil {
		t.Fatalf("SaveCurrent(first) error = %v", err)
	}

	second := manager.CreateSession("second-model")
	second.ID = "second-session"
	second.AddMessage(types.Message{Role: types.RoleUser, Content: []types.ContentBlock{types.TextBlock{Text: "second"}}})
	secondPath, err := manager.SaveCurrent()
	if err != nil {
		t.Fatalf("SaveCurrent(second) error = %v", err)
	}

	older := time.Now().Add(-2 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(firstPath, older, older); err != nil {
		t.Fatalf("touch first session: %v", err)
	}
	if err := os.Chtimes(secondPath, newer, newer); err != nil {
		t.Fatalf("touch second session: %v", err)
	}

	sessions, err := manager.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("sessions = %d, want 2", len(sessions))
	}

	read, err := manager.ReadSession("first-session")
	if err != nil {
		t.Fatalf("ReadSession(first) error = %v", err)
	}
	if read.ID != "first-session" {
		t.Fatalf("read ID = %q", read.ID)
	}
	if manager.GetCurrent().ID != "second-session" {
		t.Fatalf("ReadSession changed current session to %q", manager.GetCurrent().ID)
	}

	resumed, ok := manager.ResumeLatestSession()
	if !ok {
		t.Fatal("ResumeLatestSession() did not find a session")
	}
	if resumed.ID != "second-session" || manager.GetCurrent().ID != "second-session" {
		t.Fatalf("resumed/current = %q/%q, want second-session", resumed.ID, manager.GetCurrent().ID)
	}

	if err := manager.DeleteSession("first-session"); err != nil {
		t.Fatalf("DeleteSession(first) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(sessionsDir, "first-session.json")); !os.IsNotExist(err) {
		t.Fatalf("deleted session still exists or stat failed unexpectedly: %v", err)
	}
}

func TestSessionManagerRejectsInvalidLifecycleOperations(t *testing.T) {
	t.Setenv("AGENT_HARNESS_SESSION_DIR", t.TempDir())

	manager, err := NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	if _, err := manager.SaveCurrent(); err == nil {
		t.Fatal("expected SaveCurrent to reject missing active session")
	}

	session := manager.CreateSession("test-model")
	session.ID = "active-session"
	if _, err := manager.SaveCurrent(); err != nil {
		t.Fatalf("SaveCurrent(active) error = %v", err)
	}

	if err := manager.DeleteSession("active-session"); err == nil || !strings.Contains(err.Error(), "cannot delete the active session") {
		t.Fatalf("DeleteSession(active) error = %v, want active-session protection", err)
	}

	if err := manager.DeleteSession("missing-session"); err == nil || !strings.Contains(err.Error(), "session not found") {
		t.Fatalf("DeleteSession(missing) error = %v, want not found", err)
	}
}
