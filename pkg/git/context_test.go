package git

import (
	"os"
	"testing"
)

// TestGetContextForDirParallelCalls verifies the parallel collection
// produces a coherent context: the root/branch/commit are populated,
// and HasChanges is derived from the single status call (the dedupe
// that replaced the second `git status`).
func TestGetContextForDirParallelCalls(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	ctx, err := GetContextForDir(dir)
	if err != nil {
		t.Fatalf("GetContextForDir: %v", err)
	}
	if !ctx.IsRepo {
		t.Fatalf("expected %s to be a git repo", dir)
	}
	if ctx.Root == "" || ctx.Branch == "" || ctx.Commit == "" {
		t.Fatalf("incomplete context: root=%q branch=%q commit=%q", ctx.Root, ctx.Branch, ctx.Commit)
	}

	// HasChanges must agree with the status list: both come from the
	// same single `git status --short` call.
	if want := len(ctx.StatusFiles) > 0; ctx.HasChanges != want {
		t.Fatalf("HasChanges = %v, want %v (StatusFiles=%d)", ctx.HasChanges, want, len(ctx.StatusFiles))
	}
}
