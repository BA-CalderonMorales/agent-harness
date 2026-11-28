package builtin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

func TestGrepBoundsAccumulatedOutputBytesWhileSupporting80KiBLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big-lines.txt")
	line := strings.Repeat("x", 80*1024) + "NEEDLE"
	var b strings.Builder
	for i := 0; i < 12; i++ {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}

	res, err := GrepTool.Call(map[string]any{"pattern": "NEEDLE", "path": path}, tools.Context{AbortController: context.Background()}, nil, nil)
	if err != nil {
		t.Fatalf("GrepTool.Call() error = %v", err)
	}
	out := res.Data.(string)
	if !strings.Contains(out, "NEEDLE") {
		t.Fatal("expected at least the first 80 KiB matching line to be preserved")
	}
	if !strings.Contains(out, "[Output truncated:") {
		t.Fatalf("expected byte-budget truncation marker, got len=%d", len(out))
	}
	if len(out) > maxGrepOutputBytes+512 {
		t.Fatalf("grep result exceeded output byte budget: len=%d, limit=%d", len(out), maxGrepOutputBytes)
	}
}
