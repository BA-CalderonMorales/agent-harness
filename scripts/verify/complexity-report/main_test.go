package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeSkipsHiddenAndTemporaryDirs(t *testing.T) {
	root := t.TempDir()

	writeGo := func(rel, body string) {
		t.Helper()
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	writeGo("pkg/live/live.go", "package live\n\nfunc Real() int { return 1 }\n")
	writeGo(".claude/worktrees/agent-1/pkg/live/live.go", "package live\n\nfunc Ghost() int { return 2 }\n")
	writeGo(".workspace/scratch/x.go", "package scratch\n\nfunc Scratch() int { return 3 }\n")
	writeGo("tmp/experiment/x.go", "package exp\n\nfunc Tmp() int { return 4 }\n")

	rep, err := analyze(root, 10, 400)
	if err != nil {
		t.Fatalf("analyze() error = %v", err)
	}
	if rep.TotalSourceFiles != 1 {
		t.Fatalf("TotalSourceFiles = %d, want 1 (packages=%v)", rep.TotalSourceFiles, rep.Packages)
	}
	if rep.Functions != 1 {
		t.Fatalf("Functions = %d, want 1", rep.Functions)
	}
	if len(rep.TopComplexity) != 1 || rep.TopComplexity[0].Name != "Real" {
		t.Fatalf("TopComplexity = %#v, want only Real", rep.TopComplexity)
	}
}
