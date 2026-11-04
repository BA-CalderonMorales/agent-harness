package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"

	tea "github.com/charmbracelet/bubbletea"
)

func TestEventLogObservesWithoutReplacingMessages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	l, err := openEventLog(path)
	if err != nil {
		t.Fatal(err)
	}
	defer l.file.Close()
	for _, msg := range []tea.Msg{AgentStartMsg{}, AgentChunkMsg{Text: "hello"}, AgentDoneMsg{FullResponse: "hello"}} {
		if got := l.filter(nil, msg); got != msg {
			t.Fatalf("replaced %T", msg)
		}
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	d := json.NewDecoder(f)
	for _, kind := range []string{"start", "text", "complete"} {
		var r map[string]any
		if err := d.Decode(&r); err != nil {
			t.Fatal(err)
		}
		if r["type"] != kind || r["turn"] != float64(1) {
			t.Fatalf("record: %v", r)
		}
	}
	info, _ := f.Stat()
	if info.Mode().Perm() != 0600 {
		t.Fatalf("permissions: %v", info.Mode())
	}
	if _, err := openEventLog(path); err == nil {
		t.Fatal("overwrote existing evidence")
	}
}

func TestEventLogApprovalUsesItsOwnInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	l, err := openEventLog(path)
	if err != nil {
		t.Fatal(err)
	}
	defer l.file.Close()
	l.filter(nil, AgentToolStartMsg{ToolID: "other", ToolName: "write", Input: map[string]any{"file_path": "names.py"}})
	l.filter(nil, ApprovalRequestMsg{
		Request: approval.NewApprovalRequest(approval.CommandInfo{ID: "request", ToolName: "write"}),
		Input:   map[string]any{"file_path": "test_names.py"},
	})
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	d := json.NewDecoder(f)
	var r map[string]any
	if err := d.Decode(&r); err != nil {
		t.Fatal(err)
	}
	if err := d.Decode(&r); err != nil {
		t.Fatal(err)
	}
	if r["id"] != "request" || r["input"].(map[string]any)["file_path"] != "test_names.py" {
		t.Fatalf("approval confused with another proposal: %v", r)
	}
}

func TestEventLogFailureStopsInsteadOfSilentlyLosingEvidence(t *testing.T) {
	l, err := openEventLog(filepath.Join(t.TempDir(), "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	l.file.Close()
	if _, ok := l.filter(nil, AgentStartMsg{}).(tea.QuitMsg); !ok || l.err == nil {
		t.Fatal("log failure was not surfaced")
	}
	if l, err := openEventLog(""); l != nil || err != nil {
		t.Fatal("logging must be opt-in")
	}
}
