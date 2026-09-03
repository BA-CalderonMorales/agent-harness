package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResumeLatestSessionEmptyLocalHarness(t *testing.T) {
	sessionsDir := t.TempDir()
	t.Setenv("AGENT_HARNESS_SESSION_DIR", sessionsDir)

	manager, err := NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}
	if !strings.HasPrefix(manager.GetSessionsDir(), sessionsDir) {
		t.Fatalf("sessions dir = %q, want a project directory under %q", manager.GetSessionsDir(), sessionsDir)
	}

	resumed, ok := manager.ResumeLatestSession()
	if ok || resumed != nil {
		t.Fatalf("ResumeLatestSession() = (%v, %v), want nil false", resumed, ok)
	}
	if current := manager.GetCurrent(); current != nil {
		t.Fatalf("current session = %#v, want nil", current)
	}
}

func TestResumeLatestSessionIgnoresNonSessionFiles(t *testing.T) {
	sessionsDir := t.TempDir()
	t.Setenv("AGENT_HARNESS_SESSION_DIR", sessionsDir)

	if err := os.WriteFile(filepath.Join(sessionsDir, "notes.txt"), []byte("not a session"), 0644); err != nil {
		t.Fatalf("write non-session file: %v", err)
	}

	manager, err := NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	session := manager.CreateSession("local-model")
	session.ID = "local-session"
	if _, err := manager.SaveCurrent(); err != nil {
		t.Fatalf("SaveCurrent() error = %v", err)
	}

	resumed, ok := manager.ResumeLatestSession()
	if !ok {
		t.Fatal("ResumeLatestSession() did not resume saved session")
	}
	if resumed.ID != "local-session" || manager.GetCurrent().ID != "local-session" {
		t.Fatalf("resumed/current = %q/%q, want local-session", resumed.ID, manager.GetCurrent().ID)
	}
}
