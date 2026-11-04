package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools/builtin"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func TestWriteCannotTruncateFromMissingContentAtEitherPolicyBoundary(t *testing.T) {
	for _, changedByPolicy := range []bool{false, true} {
		t.Run(map[bool]string{false: "model omitted content", true: "policy removed content"}[changedByPolicy], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "names.py")
			original := "def normalize_name(value):\n    raise NotImplementedError()\n"
			if err := os.WriteFile(path, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			input := map[string]any{"file_path": path, "old_string": "old", "new_string": "replacement"}
			if changedByPolicy {
				input["content"] = "valid replacement"
			}
			policy := func(string, map[string]any, tools.Context) (tools.PermissionDecision, error) {
				return tools.PermissionDecision{Behavior: tools.Allow, UpdatedInput: map[string]any{"file_path": path}}, nil
			}
			_, err := runSingleTool(edgeToolContext([]tools.Tool{builtin.FileWriteTool}), types.ToolUseBlock{
				ID: "bad-write", Name: "write", Input: input,
			}, types.Message{}, []tools.Tool{builtin.FileWriteTool}, policy, nil)
			if err == nil || !strings.Contains(err.Error(), "content") {
				t.Fatalf("missing-content write was not rejected: %v", err)
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != original {
				t.Fatalf("invalid write damaged file: %q, %v", data, err)
			}
		})
	}
}
