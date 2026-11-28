package state

import (
	"os"
	"runtime"
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
	if _, err := manager.ReadSession("first-session"); err == nil {
		t.Fatal("deleted session still loads")
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

// TestListSessionsWithSizeOrderAndSizes: /cleanup's listing backend —
// oldest first, sizes present, and the active session's delete refusal
// still holds through the same path.
func TestListSessionsWithSizeOrderAndSizes(t *testing.T) {
	sessionsDir := t.TempDir()
	t.Setenv("AGENT_HARNESS_SESSION_DIR", sessionsDir)

	manager, err := NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	first := manager.CreateSession("m")
	first.ID = "aaa-older"
	first.AddMessage(types.Message{Role: types.RoleUser, Content: []types.ContentBlock{types.TextBlock{Text: strings.Repeat("x", 2000)}}})
	firstPath, err := manager.SaveCurrent()
	if err != nil {
		t.Fatalf("SaveCurrent(first) error = %v", err)
	}

	second := manager.CreateSession("m")
	second.ID = "bbb-newer"
	second.AddMessage(types.Message{Role: types.RoleUser, Content: []types.ContentBlock{types.TextBlock{Text: "y"}}})
	if _, err := manager.SaveCurrent(); err != nil {
		t.Fatalf("SaveCurrent(second) error = %v", err)
	}

	older := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(firstPath, older, older); err != nil {
		t.Fatalf("touch first session: %v", err)
	}

	rows, err := manager.ListSessionsWithSize()
	if err != nil {
		t.Fatalf("ListSessionsWithSize() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].ID != "aaa-older" || rows[1].ID != "bbb-newer" {
		t.Fatalf("order = [%s, %s], want oldest first", rows[0].ID, rows[1].ID)
	}
	for _, r := range rows {
		if r.SizeBytes <= 0 {
			t.Fatalf("row %s has size %d, want >0", r.ID, r.SizeBytes)
		}
	}
	// The bigger session reports the bigger size.
	if rows[0].SizeBytes <= rows[1].SizeBytes {
		t.Fatalf("sizes not ordered with content: %d <= %d", rows[0].SizeBytes, rows[1].SizeBytes)
	}

	// Active-session guard through the cleanup path: the current
	// session is bbb-newer; deleting it must be refused.
	if err := manager.DeleteSession("bbb-newer"); err == nil || !strings.Contains(err.Error(), "cannot delete the active session") {
		t.Fatalf("DeleteSession(active) error = %v, want refusal", err)
	}
}

func TestListSessionsWithIssuesRetainsReadableSessions(t *testing.T) {
	manager, err := NewSessionManagerWithDir(t.TempDir())
	if err != nil {
		t.Fatalf("NewSessionManagerWithDir() error = %v", err)
	}
	session := manager.CreateSession("model")
	session.ID = "readable-session"
	path, err := manager.SaveCurrent()
	if err != nil {
		t.Fatalf("SaveCurrent() error = %v", err)
	}
	if err := os.WriteFile(path+"-damaged.jsonl", []byte("not json\n"), 0600); err != nil {
		t.Fatalf("write damaged session: %v", err)
	}

	sessions, unreadable, err := manager.ListSessionsWithIssues()
	if err != nil {
		t.Fatalf("ListSessionsWithIssues() error = %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != session.ID {
		t.Fatalf("sessions = %#v, want readable session only", sessions)
	}
	if unreadable != 1 {
		t.Fatalf("unreadable = %d, want 1", unreadable)
	}
}

func TestSetCurrentPreservesJournalCacheAndCompactionStillRewrites(t *testing.T) {
	manager, err := NewSessionManagerWithDir(t.TempDir())
	if err != nil {
		t.Fatalf("NewSessionManagerWithDir() error = %v", err)
	}

	session := manager.CreateSession("test-model")
	session.AddMessage(types.Message{Role: types.RoleUser, Content: []types.ContentBlock{types.TextBlock{Text: "one"}}})
	session.AddMessage(types.Message{Role: types.RoleAssistant, Content: []types.ContentBlock{types.TextBlock{Text: "two"}}})
	path, err := manager.SaveCurrent()
	if err != nil {
		t.Fatalf("SaveCurrent() error = %v", err)
	}

	// Re-setting the active session must preserve journal state so subsequent
	// appends do not re-read the whole file from disk.
	manager.SetCurrent(session)
	if manager.journalID != session.ID || manager.appendOffset != 2 {
		t.Fatalf("SetCurrent(same) wiped journal: id=%q offset=%d", manager.journalID, manager.appendOffset)
	}

	// Shrinking messages (compaction) on the same session must still rewrite.
	session.Messages = session.Messages[1:]
	manager.SetCurrent(session)
	if _, err := manager.SaveCurrent(); err != nil {
		t.Fatalf("SaveCurrent(compacted) error = %v", err)
	}
	loaded, err := loadSessionFile(path)
	if err != nil {
		t.Fatalf("loadSessionFile() error = %v", err)
	}
	if len(loaded.Messages) != 1 {
		t.Fatalf("compacted session has %d messages on disk, want 1", len(loaded.Messages))
	}
}

func TestFormatSessionReportHandlesShortID(t *testing.T) {
	manager, err := NewSessionManagerWithDir(t.TempDir())
	if err != nil {
		t.Fatalf("NewSessionManagerWithDir() error = %v", err)
	}
	session := manager.CreateSession("test-model")
	session.ID = "short"
	report := manager.FormatSessionReport()
	if !strings.Contains(report, "short") {
		t.Fatalf("FormatSessionReport() = %q, want short ID", report)
	}
}

func TestCountMessageEventsIgnoresEmbeddedPayloadMatches(t *testing.T) {
	path := t.TempDir() + "/s.jsonl"
	content := strings.Join([]string{
		`{"type":"header","id":"abc"}`,
		`{"type":"message","message":{"role":"user","content":[{"text":"literal {\"type\":\"message\"} inside text"}]}}`,
		`{"type":"meta","meta":{"model":"{\"type\":\"message\"}"}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	n, err := countMessageEventsChecked(path)
	if err != nil {
		t.Fatalf("countMessageEventsChecked() error = %v", err)
	}
	if n != 1 {
		t.Fatalf("countMessageEventsChecked() = %d, want 1", n)
	}
}

func TestAppendRollbackStatFailureMarksNeedsRewrite(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unlinking an open file path to fail os.Stat during rollback is POSIX-specific")
	}
	dir := t.TempDir()
	manager, err := NewSessionManagerWithDir(dir)
	if err != nil {
		t.Fatalf("NewSessionManagerWithDir() error = %v", err)
	}
	session := manager.CreateSession("test-model")
	session.AddMessage(types.Message{Role: types.RoleUser, Content: []types.ContentBlock{types.TextBlock{Text: "first"}}})
	path, err := manager.SaveCurrent()
	if err != nil {
		t.Fatalf("initial SaveCurrent() error = %v", err)
	}

	session.AddMessage(types.Message{Role: types.RoleAssistant, Content: []types.ContentBlock{types.TextBlock{Text: "second"}}})
	manager.appendFault = func() error {
		if err := os.Remove(path); err != nil {
			t.Errorf("remove during appendFault: %v", err)
		}
		return os.ErrPermission
	}
	if _, err := manager.SaveCurrent(); err == nil {
		t.Fatal("SaveCurrent() = nil, want injected append error")
	}
	if !manager.needsRewrite {
		t.Fatal("needsRewrite = false after rollback os.Stat failure, want true")
	}
	manager.appendFault = nil
	if _, err := manager.SaveCurrent(); err != nil {
		t.Fatalf("recovery SaveCurrent() error = %v", err)
	}
	if manager.needsRewrite {
		t.Fatal("needsRewrite stayed true after successful recovery rewrite")
	}
	loaded, err := loadSessionFile(path)
	if err != nil {
		t.Fatalf("loadSessionFile() error = %v", err)
	}
	if len(loaded.Messages) != 2 {
		t.Fatalf("loaded messages = %d, want 2 without duplicates", len(loaded.Messages))
	}
}
