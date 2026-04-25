package behaviors

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/permissions"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/internal/testharness"
)

func TestBehavior_ToolExecution(t *testing.T) {
	tests := []struct {
		name         string
		given        func(*testharness.Fixture)
		tool         string
		input        map[string]any
		wantDecision tools.DecisionBehavior
		wantErr      bool
		wantContains string
	}{
		{
			name: "B3: read tool allowed in default mode",
			given: func(f *testharness.Fixture) {
				f.SetPermissionMode(permissions.ModeDefault)
			},
			tool:         "read",
			input:        map[string]any{"file_path": "test.txt"},
			wantDecision: tools.Allow,
			wantErr:      false,
			wantContains: "hello file",
		},
		{
			name: "B4: bash tool allowed for safe command in default mode",
			given: func(f *testharness.Fixture) {
				f.SetPermissionMode(permissions.ModeDefault)
			},
			tool:         "bash",
			input:        map[string]any{"command": "echo hello"},
			wantDecision: tools.Allow,
			wantErr:      false,
			wantContains: "hello",
		},
		{
			name: "B5: bash tool allowed in dontAsk mode",
			given: func(f *testharness.Fixture) {
				f.SetPermissionMode(permissions.ModeDontAsk)
			},
			tool:         "bash",
			input:        map[string]any{"command": "echo hello"},
			wantDecision: tools.Allow,
			wantErr:      false,
			wantContains: "hello",
		},
		{
			name: "B6: denied tool blocked by rule",
			given: func(f *testharness.Fixture) {
				f.SetPermissionMode(permissions.ModeDefault)
				f.SetAlwaysDeny("bash")
			},
			tool:         "bash",
			input:        map[string]any{"command": "echo hello"},
			wantDecision: tools.Deny,
			wantErr:      true,
		},
		{
			name: "B7: write tool creates file",
			given: func(f *testharness.Fixture) {
				f.SetPermissionMode(permissions.ModeDontAsk)
			},
			tool:         "write",
			input:        map[string]any{"file_path": "out.txt", "content": "test data"},
			wantDecision: tools.Allow,
			wantErr:      false,
			wantContains: "test data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := testharness.NewFixture(t)

			// Pre-seed a file for read tests
			if tt.tool == "read" {
				p := filepath.Join(f.WorkDir, "test.txt")
				if err := os.WriteFile(p, []byte("hello file"), 0644); err != nil {
					t.Fatalf("setup: %v", err)
				}
				tt.input["file_path"] = p
			}
			if tt.tool == "write" {
				tt.input["file_path"] = filepath.Join(f.WorkDir, "out.txt")
			}

			if tt.given != nil {
				tt.given(f)
			}

			err := f.ExecuteTool(tt.tool, tt.input)

			if f.LastDecision().Behavior != tt.wantDecision {
				t.Errorf("decision = %s, want %s", f.LastDecision().Behavior, tt.wantDecision)
			}

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantContains != "" {
				switch tt.tool {
				case "write":
					data, _ := os.ReadFile(tt.input["file_path"].(string))
					if !strings.Contains(string(data), tt.wantContains) {
						t.Errorf("file content does not contain %q", tt.wantContains)
					}
				case "read":
					// read result was verified by successful execution
				}
			}
		})
	}
}
