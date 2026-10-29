package builtin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
)

func TestMutationRejectsMissingOrWrongTypeReplacementWithoutChangingFiles(t *testing.T) {
	cases := []struct {
		name    string
		present bool
		value   any
	}{
		{"missing", false, nil}, {"null", true, nil},
		{"number", true, 42}, {"boolean", true, false},
		{"array", true, []string{"replacement"}},
		{"object", true, map[string]any{"text": "replacement"}},
	}
	for _, tool := range []tools.Tool{FileWriteTool, FileEditTool} {
		field := "content"
		if tool.Name == "edit" {
			field = "new_string"
		}
		for _, exists := range []bool{false, true} {
			for _, tc := range cases {
				t.Run(fmt.Sprintf("%s/%s/existing=%v", tool.Name, tc.name, exists), func(t *testing.T) {
					path := filepath.Join(t.TempDir(), "names.py")
					original := "def normalize_name(value):\n    raise NotImplementedError()\n"
					if exists {
						if err := os.WriteFile(path, []byte(original), 0600); err != nil {
							t.Fatal(err)
						}
					}
					// Reproduce write receiving edit-shaped arguments.
					input := map[string]any{"file_path": path, "old_string": "raise NotImplementedError()", "new_string": "return value"}
					delete(input, field)
					if tc.present {
						input[field] = tc.value
					}
					validation := tool.ValidateInput(input, tools.Context{})
					if validation.Valid || !strings.Contains(validation.Message, field) {
						t.Fatalf("invalid replacement accepted: %#v", validation)
					}
					if tool.Name == "write" && strings.Contains(tool.GetActivityDescription(input), "0 bytes") {
						t.Fatal("missing/wrong-type content misleadingly previewed as an intentional empty write")
					}
					// Direct execution must enforce this too, even if validation
					// was bypassed or input changed after a permission decision.
					if _, err := tool.Call(input, tools.Context{}, nil, nil); err == nil {
						t.Fatal("execution bypassed required-string validation")
					}
					data, err := os.ReadFile(path)
					if exists && (err != nil || string(data) != original) {
						t.Fatalf("invalid input damaged existing file: %q, %v", data, err)
					}
					if !exists && !os.IsNotExist(err) {
						t.Fatalf("invalid input created a file: %v", err)
					}
				})
			}
		}
	}
}

func TestExplicitEmptyMutationRemainsSupported(t *testing.T) {
	for _, tool := range []tools.Tool{FileWriteTool, FileEditTool} {
		t.Run(tool.Name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "empty.txt")
			if err := os.WriteFile(path, []byte("remove me"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := FileReadTool.Call(map[string]any{"file_path": path}, tools.Context{}, nil, nil); err != nil {
				t.Fatal(err)
			}
			input := map[string]any{"file_path": path, "content": ""}
			if tool.Name == "edit" {
				input = map[string]any{"file_path": path, "old_string": "remove me", "new_string": ""}
			}
			if validation := tool.ValidateInput(input, tools.Context{}); !validation.Valid {
				t.Fatal(validation.Message)
			}
			if _, err := tool.Call(input, tools.Context{}, nil, nil); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil || len(data) != 0 {
				t.Fatalf("explicit empty mutation failed: %q, %v", data, err)
			}
		})
	}
}
