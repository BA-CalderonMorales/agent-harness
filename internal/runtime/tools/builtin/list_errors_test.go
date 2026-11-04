package builtin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
)

func TestDirectoryListingFailuresAreErrorsWithCorrectiveGuidance(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "test_names.py")
	if err := os.WriteFile(file, []byte("tests"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tool := range []tools.Tool{ListDirectoryTool, LsRecursiveTool} {
		for _, path := range []string{filepath.Join(root, "missing"), file, "ls " + file + " 2>/dev/null || ls *.py"} {
			t.Run(tool.Name+"/"+path, func(t *testing.T) {
				_, err := tool.Call(map[string]any{"path": path}, tools.Context{}, nil, nil)
				if err == nil {
					t.Fatal("filesystem failure reported as successful tool execution")
				}
				for _, hint := range []string{"literal directory path", "file_path", "bash"} {
					if !strings.Contains(err.Error(), hint) {
						t.Fatalf("missing %q guidance: %v", hint, err)
					}
				}
			})
		}
	}
}
