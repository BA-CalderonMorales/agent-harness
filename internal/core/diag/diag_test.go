package diag

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiagWritesSiteTaggedJSONL(t *testing.T) {
	dir := t.TempDir()
	SetDir(dir)
	defer func() { SetDir("") }()

	Error("session.save.submit", os.ErrPermission)
	Panic("tui.app_update", "boom")

	data, err := os.ReadFile(filepath.Join(dir, "today.log"))
	// The daily file name is UTC-date based; find whatever .log exists.
	if err != nil {
		entries, readErr := os.ReadDir(dir)
		if readErr != nil || len(entries) == 0 {
			t.Fatalf("no diag log written: %v", err)
		}
		data, err = os.ReadFile(filepath.Join(dir, entries[0].Name()))
		if err != nil {
			t.Fatalf("read diag log: %v", err)
		}
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 JSONL entries, got %d", len(lines))
	}

	var errEntry, panicEntry Entry
	if err := json.Unmarshal([]byte(lines[0]), &errEntry); err != nil {
		t.Fatalf("unmarshal error entry: %v", err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &panicEntry); err != nil {
		t.Fatalf("unmarshal panic entry: %v", err)
	}

	if errEntry.Level != "error" || errEntry.Site != "session.save.submit" {
		t.Errorf("error entry = %+v", errEntry)
	}
	if errEntry.Message != "permission denied" {
		t.Errorf("error message = %q", errEntry.Message)
	}
	if panicEntry.Level != "panic" || panicEntry.Site != "tui.app_update" {
		t.Errorf("panic entry = %+v", panicEntry)
	}
	if panicEntry.Message != "boom" {
		t.Errorf("panic message = %q", panicEntry.Message)
	}
	if !strings.Contains(panicEntry.Stack, "TestDiagWritesSiteTaggedJSONL") {
		t.Error("panic entry should carry the goroutine stack")
	}
}

func TestDiagNeverFailsTheCaller(t *testing.T) {
	SetDir(filepath.Join(t.TempDir(), "nested", "deep"))
	defer func() { SetDir("") }()

	// All of these must not panic even when the directory cannot be
	// created (path conflicts with a file).
	Error("test.site", os.ErrClosed)
	Panic("test.site", "x")
}
