package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/state"
)

func TestGetSessionInfosSurfacesUnreadableFileWithoutPrivateDetails(t *testing.T) {
	dir := t.TempDir()
	manager, err := state.NewSessionManagerWithDir(dir)
	if err != nil {
		t.Fatalf("NewSessionManagerWithDir() error = %v", err)
	}
	privateText := "secret transcript content"
	projectSlug := strings.ReplaceAll(strings.Trim(dir, "/"), "/", "-")
	projectDir := filepath.Join(dir, projectSlug)
	if err := os.MkdirAll(projectDir, 0700); err != nil {
		t.Fatalf("create disposable project session dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "2026-01-01T00-00-00_damaged.jsonl"), []byte(privateText+"\n"), 0600); err != nil {
		t.Fatalf("write disposable damaged session: %v", err)
	}
	app := &App{sessionManager: manager}
	infos := app.getSessionInfos()
	if len(infos) != 1 || infos[0].UnreadableCount != 1 {
		t.Fatalf("session infos = %#v, want one safe warning row", infos)
	}
	if infos[0].ID != "" || infos[0].Title != "" {
		t.Fatalf("warning row leaked session metadata: %#v", infos[0])
	}
}

func TestEnsureCurrentSessionAddsMissingSession(t *testing.T) {
	existing := []state.SessionMetadata{{ID: "aaaa"}}
	current := state.NewSession("test-model")

	got := ensureCurrentSession(existing, current)
	if len(got) != 2 {
		t.Fatalf("got %d sessions, want 2 (current appended)", len(got))
	}
	if got[1].ID != current.ID {
		t.Fatalf("appended session = %s, want current %s", got[1].ID, current.ID)
	}
}

func TestEnsureCurrentSessionDedupes(t *testing.T) {
	current := state.NewSession("test-model")
	existing := []state.SessionMetadata{
		{ID: "aaaa"},
		current.GetMetadata(),
	}

	got := ensureCurrentSession(existing, current)
	if len(got) != 2 {
		t.Fatalf("got %d sessions, want 2 (current already listed, no duplicate)", len(got))
	}
}

func TestEnsureCurrentSessionNilSafe(t *testing.T) {
	existing := []state.SessionMetadata{{ID: "aaaa"}}
	if got := ensureCurrentSession(existing, nil); len(got) != 1 {
		t.Fatalf("nil current must not add entries, got %d", len(got))
	}
	empty := state.NewSession("")
	empty.ID = ""
	if got := ensureCurrentSession(existing, empty); len(got) != 1 {
		t.Fatalf("empty-id current must not add entries, got %d", len(got))
	}
}
