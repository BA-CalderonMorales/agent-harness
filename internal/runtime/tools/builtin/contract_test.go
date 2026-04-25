package builtin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
)

func TestBuiltinTools_HappyPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Pre-seed a file for read/grep tests
	testFile := filepath.Join(tmpDir, "hello.txt")
	if err := os.WriteFile(testFile, []byte("hello world\nline two\n"), 0644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Pre-seed a nested dir for glob/ls_recursive tests
	nested := filepath.Join(tmpDir, "sub", "deep")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "a.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	ctx := tools.Context{
		AbortController: context.Background(),
		GlobLimits:      tools.GlobLimits{MaxResults: 100},
	}

	tests := []struct {
		name         string
		tool         tools.Tool
		input        map[string]any
		wantContains string
		wantErr      bool
	}{
		{
			name:         "read existing file",
			tool:         FileReadTool,
			input:        map[string]any{"file_path": testFile},
			wantContains: "hello world",
		},
		{
			name:         "read with offset and limit",
			tool:         FileReadTool,
			input:        map[string]any{"file_path": testFile, "offset": float64(1), "limit": float64(1)},
			wantContains: "line two",
		},
		{
			name:         "write new file",
			tool:         FileWriteTool,
			input:        map[string]any{"file_path": filepath.Join(tmpDir, "new.txt"), "content": "new content"},
			wantContains: "new.txt",
		},
		{
			name:         "glob finds files",
			tool:         GlobTool,
			input:        map[string]any{"pattern": "*.txt", "path": tmpDir},
			wantContains: "hello.txt",
		},
		{
			name:         "grep finds match",
			tool:         GrepTool,
			input:        map[string]any{"pattern": "hello", "path": tmpDir},
			wantContains: "hello.txt:1:hello world",
		},
		{
			name:         "ls_recursive lists directory",
			tool:         LsRecursiveTool,
			input:        map[string]any{"path": tmpDir, "depth": float64(3)},
			wantContains: "a.go",
		},
		{
			name:         "bash echo",
			tool:         BashTool,
			input:        map[string]any{"command": "echo hi"},
			wantContains: "hi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.tool.Call(tt.input, ctx, nil, nil)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			s, _ := result.Data.(string)
			if !strings.Contains(s, tt.wantContains) {
				t.Errorf("result %q does not contain %q", s, tt.wantContains)
			}
		})
	}
}
