package main

import (
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/state"
)

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
