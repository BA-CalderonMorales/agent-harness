package commands

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// seedSessions builds a deterministic listing: three sessions, oldest
// first, distinct sizes.
func seedSessions() ([]CleanupSession, error) {
	return []CleanupSession{
		{ID: "old-big", UpdatedAt: time.Now().Add(-48 * time.Hour), MessageCount: 400, SizeBytes: 3 << 20},
		{ID: "mid", UpdatedAt: time.Now().Add(-24 * time.Hour), MessageCount: 120, SizeBytes: 512 << 10},
		{ID: "new-small", UpdatedAt: time.Now().Add(-1 * time.Hour), MessageCount: 3, SizeBytes: 900},
	}, nil
}

// TestCleanupListsSessionsWithoutDeleting: bare /cleanup lists by
// size/age and deletes nothing.
func TestCleanupListsSessionsWithoutDeleting(t *testing.T) {
	deleted := 0
	out, err := CleanupHandler(seedSessions, func(id string) error {
		deleted++
		return nil
	})("")
	if err != nil {
		t.Fatalf("/cleanup error = %v", err)
	}
	if deleted != 0 {
		t.Fatalf("bare /cleanup deleted %d sessions, want 0", deleted)
	}
	for _, want := range []string{"old-big", "mid", "new-small", "3.0M", "512.0K", "900B", "--confirm"} {
		if !strings.Contains(out, want) {
			t.Fatalf("listing missing %q:\n%s", want, out)
		}
	}
	// Oldest first.
	if strings.Index(out, "old-big") > strings.Index(out, "new-small") {
		t.Fatalf("listing not oldest-first:\n%s", out)
	}
}

// TestCleanupConfirmDeletesNamedSessions: /cleanup <id> --confirm
// deletes exactly the named sessions through the delete function.
func TestCleanupConfirmDeletesNamedSessions(t *testing.T) {
	var deleted []string
	out, err := CleanupHandler(seedSessions, func(id string) error {
		deleted = append(deleted, id)
		return nil
	})("old-big new-small --confirm")
	if err != nil {
		t.Fatalf("/cleanup --confirm error = %v", err)
	}
	if len(deleted) != 2 || deleted[0] != "old-big" || deleted[1] != "new-small" {
		t.Fatalf("deleted = %v, want [old-big new-small]", deleted)
	}
	if !strings.Contains(out, "deleted 2, failed 0") {
		t.Fatalf("summary missing:\n%s", out)
	}
}

// TestCleanupConfirmAllSkipsActiveViaDeleteFn: --all --confirm asks the
// delete function for every session; the active-session guard lives in
// SessionManager.DeleteSession, surfaced here as a failed deletion.
func TestCleanupConfirmAllSurfacesActiveGuard(t *testing.T) {
	var deleted, refused []string
	out, err := CleanupHandler(seedSessions, func(id string) error {
		if id == "mid" { // pretend "mid" is the active session
			refused = append(refused, id)
			return errActiveSession{}
		}
		deleted = append(deleted, id)
		return nil
	})("--all --confirm")
	if err != nil {
		t.Fatalf("/cleanup --all --confirm error = %v", err)
	}
	if len(deleted) != 2 || len(refused) != 1 {
		t.Fatalf("deleted=%v refused=%v", deleted, refused)
	}
	if !strings.Contains(out, "deleted 2, failed 1") || !strings.Contains(out, "mid") {
		t.Fatalf("summary missing refusal detail:\n%s", out)
	}
}

// TestCleanupNoConfirmNoDelete: naming IDs without --confirm must not
// delete — the dry-run only previews.
func TestCleanupNoConfirmNoDelete(t *testing.T) {
	deleted := 0
	out, err := CleanupHandler(seedSessions, func(id string) error {
		deleted++
		return nil
	})("old-big")
	if err != nil {
		t.Fatalf("/cleanup <id> error = %v", err)
	}
	if deleted != 0 {
		t.Fatalf("dry-run deleted %d sessions", deleted)
	}
	if !strings.Contains(out, "Rerun with --confirm") {
		t.Fatalf("dry-run missing hint:\n%s", out)
	}
}

// TestCleanupEmptyRegistry: no sessions → friendly message, no error.
func TestCleanupEmptyRegistry(t *testing.T) {
	out, err := CleanupHandler(func() ([]CleanupSession, error) { return nil, nil }, func(id string) error { return nil })("")
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(out, "No saved sessions") {
		t.Fatalf("unexpected output: %q", out)
	}
}

// errActiveSession mimics the SessionManager active-session refusal.
type errActiveSession struct{}

func (errActiveSession) Error() string { return "cannot delete the active session" }

// TestCleanupListingErrorSurfaces: a listing failure must reach the
// user as a command error — never disguised as "no saved sessions"
// (review finding on the swallowed-error wrapper).
func TestCleanupListingErrorSurfaces(t *testing.T) {
	_, err := CleanupHandler(func() ([]CleanupSession, error) {
		return nil, fmt.Errorf("permission denied")
	}, func(id string) error { return nil })("")
	if err == nil {
		t.Fatal("listing error swallowed — /cleanup reported success")
	}
	if !strings.Contains(err.Error(), "listing sessions failed") || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("error lost the cause: %v", err)
	}
}
