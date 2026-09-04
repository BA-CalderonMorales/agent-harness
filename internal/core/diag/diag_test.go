package diag

import (
	"encoding/json"
	"fmt"
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

// TestDiagCarriesCallerAndLevels pins the Splunk-shaped contract: every
// entry carries the exact file:line of the diag call, and the level
// bands (info/warning/error/panic) survive the JSONL round-trip.
func TestDiagCarriesCallerAndLevels(t *testing.T) {
	dir := t.TempDir()
	SetDir(dir)
	defer func() { SetDir("") }()

	Info("provider.ready", "2 models available")
	Warn("tui.send.drop", os.ErrDeadlineExceeded)
	Errorf("session.save.turn", "disk full after %d bytes", 42)

	entries := readAll(t, dir)
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}
	if entries[0].Level != "info" || entries[1].Level != "warning" || entries[2].Level != "error" {
		t.Fatalf("levels = %s/%s/%s", entries[0].Level, entries[1].Level, entries[2].Level)
	}
	for i, e := range entries {
		if !strings.Contains(e.Caller, "diag_test.go") {
			t.Errorf("entry %d caller = %q, want diag_test.go:<line>", i, e.Caller)
		}
		if e.Site == "" {
			t.Errorf("entry %d missing site", i)
		}
	}
	if !strings.Contains(entries[2].Message, "disk full after 42 bytes") {
		t.Errorf("formatted message = %q", entries[2].Message)
	}
}

// TestDiagRingAndSink pins the in-memory stream: Recent returns the
// capped history oldest-first, and the sink sees every entry as it is
// logged.
func TestDiagRingAndSink(t *testing.T) {
	dir := t.TempDir()
	SetDir(dir)
	defer func() { SetDir("") }()

	var seen []Entry
	SetSink(func(e Entry) { seen = append(seen, e) })
	defer SetSink(nil)

	for i := 0; i < 3; i++ {
		Infof("ring.test", "entry %d", i)
	}
	recent := Recent()
	if len(recent) < 3 {
		t.Fatalf("Recent() = %d entries, want >= 3", len(recent))
	}
	last3 := recent[len(recent)-3:]
	for i, e := range last3 {
		if !strings.Contains(e.Message, "entry "+fmt.Sprint(i)) {
			t.Fatalf("ring out of order: %+v", last3)
		}
	}
	if len(seen) != 3 || seen[2].Message != "entry 2" {
		t.Fatalf("sink saw %d entries", len(seen))
	}
}

func readAll(t *testing.T, dir string) []Entry {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("no diag log written")
	}
	data, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatalf("read diag log: %v", err)
	}
	var out []Entry
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err == nil {
			out = append(out, e)
		}
	}
	return out
}

// TestDiagSinkMustNotRecurse pins the failure surface rule: a sink that
// logs (the TUI forwards entries through Send, and Send's full-channel
// drop path logs) must not loop — the first stack overflow took down
// the whole test binary. Re-entrant entries skip the sink.
func TestDiagSinkMustNotRecurse(t *testing.T) {
	dir := t.TempDir()
	SetDir(dir)
	defer func() { SetDir("") }()

	depth := 0
	maxDepth := 0
	SetSink(func(e Entry) {
		depth++
		if depth > maxDepth {
			maxDepth = depth
		}
		if depth <= 8 {
			Errorf("sink.loop", "re-entry %d", depth)
		}
		depth--
	})
	defer SetSink(nil)

	Info("sink.test", "single spark")
	if maxDepth > 1 {
		t.Fatalf("sink recursed to depth %d, want max 1", maxDepth)
	}
}
