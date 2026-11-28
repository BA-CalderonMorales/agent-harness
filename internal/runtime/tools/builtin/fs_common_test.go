package builtin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
)

func TestCommonIgnoredDirsAreSkipped(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file that should be found
	if err := os.WriteFile(filepath.Join(tmpDir, "keep.txt"), []byte("keep me"), 0644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Create an ignored directory with a file inside
	ignoredDir := filepath.Join(tmpDir, "node_modules", "somepkg")
	if err := os.MkdirAll(ignoredDir, 0755); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ignoredDir, "ignore.txt"), []byte("ignore me"), 0644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	ctx := tools.Context{
		AbortController: t.Context(),
		GlobLimits:      tools.GlobLimits{MaxResults: 100},
	}

	// ls_recursive should NOT descend into node_modules
	t.Run("ls_recursive skips ignored dirs", func(t *testing.T) {
		result, err := LsRecursiveTool.Call(map[string]any{"path": tmpDir, "depth": int(5)}, ctx, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		s, _ := result.Data.(string)
		if s == "" {
			t.Fatal("expected non-empty result")
		}
		if contains(s, "ignore.txt") {
			t.Error("ls_recursive descended into node_modules; expected skip")
		}
		if !contains(s, "keep.txt") {
			t.Error("ls_recursive missed keep.txt")
		}
	})

	// grep should NOT search inside node_modules
	t.Run("grep skips ignored dirs", func(t *testing.T) {
		result, err := GrepTool.Call(map[string]any{"pattern": "ignore", "path": tmpDir}, ctx, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		s, _ := result.Data.(string)
		if s == "(no matches found)" {
			// Expected: "ignore me" should not be found because node_modules was skipped
			return
		}
		if contains(s, "ignore.txt") {
			t.Error("grep searched inside node_modules; expected skip")
		}
	})

	// Explicitly requesting an ignored directory as the root must still inspect
	// its contents across find, ls_recursive, and grep.
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("setup .claude failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "notes.txt"), []byte("claude-secret-needle\n"), 0o600); err != nil {
		t.Fatalf("write .claude/notes.txt failed: %v", err)
	}
	t.Run("explicit ignored root is inspected by find ls_recursive and grep", func(t *testing.T) {
		lsRes, err := LsRecursiveTool.Call(map[string]any{"path": claudeDir, "depth": 3}, ctx, nil, nil)
		if err != nil {
			t.Fatalf("ls_recursive(.claude) error = %v", err)
		}
		if !contains(lsRes.Data.(string), "notes.txt") {
			t.Fatalf("ls_recursive(.claude) hid explicit root contents: %q", lsRes.Data.(string))
		}
		findRes, err := FindTool.Call(map[string]any{"pattern": "*.txt", "path": claudeDir}, ctx, nil, nil)
		if err != nil {
			t.Fatalf("find(.claude) error = %v", err)
		}
		if !contains(findRes.Data.(string), "notes.txt") {
			t.Fatalf("find(.claude) hid explicit root contents: %q", findRes.Data.(string))
		}
		grepRes, err := GrepTool.Call(map[string]any{"pattern": "claude-secret-needle", "path": claudeDir}, ctx, nil, nil)
		if err != nil {
			t.Fatalf("grep(.claude) error = %v", err)
		}
		if !contains(grepRes.Data.(string), "notes.txt") {
			t.Fatalf("grep(.claude) hid explicit root contents: %q", grepRes.Data.(string))
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
