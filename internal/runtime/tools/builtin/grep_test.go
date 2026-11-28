package builtin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
)

func TestGrepSingleFileHonorsCanceledContext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(path, []byte("needle line 1\nneedle line 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := GrepTool.Call(map[string]any{"pattern": "needle", "path": path}, tools.Context{AbortController: ctx}, nil, nil); err == nil {
		t.Fatal("single-file GrepTool.Call() succeeded with canceled context, want context error")
	}
}
