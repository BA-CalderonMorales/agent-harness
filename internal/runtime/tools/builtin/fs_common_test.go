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
