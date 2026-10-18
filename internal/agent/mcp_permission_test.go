package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	svcmcp "github.com/BA-CalderonMorales/agent-harness/internal/runtime/services/mcp"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	mcptool "github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools/mcp"
	"github.com/BA-CalderonMorales/agent-harness/internal/testharness/mcppeer"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func TestMCPDenialAndUnavailableCheckpointNeverReachServer(t *testing.T) {
	for _, behavior := range []tools.DecisionBehavior{tools.Deny, tools.Allow} {
		t.Run(string(behavior), func(t *testing.T) {
			trace := filepath.Join(t.TempDir(), "wire.jsonl")
			command, args, err := mcppeer.Command()
			if err != nil {
				t.Fatal(err)
			}
			mgr := svcmcp.NewManager()
			t.Cleanup(func() { _ = mgr.Close() })
			if err := mgr.Connect(context.Background(), "fixture", svcmcp.McpServerConfig{
				Command: command, Args: args, Env: map[string]string{"AH_MCP_TEST_PEER": "normal", "AH_MCP_TEST_TRACE": trace},
			}); err != nil {
				t.Fatal(err)
			}
			tool := mcptool.Wrap(mgr.AllToolDefs()[0], mgr)
			_, err = runSingleTool(tools.Context{AbortController: context.Background()},
				types.ToolUseBlock{ID: "denied", Name: tool.Name, Input: map[string]any{}},
				types.Message{}, []tools.Tool{tool},
				func(string, map[string]any, tools.Context) (tools.PermissionDecision, error) {
					return tools.PermissionDecision{Behavior: behavior}, nil
				}, nil)
			if err == nil || !strings.Contains(err.Error(), "permission denied") {
				t.Fatalf("expected closed approval gate: %v", err)
			}
			wire, err := os.ReadFile(trace)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(wire), `"method":"tools/call"`) {
				t.Fatal("unapproved call reached external process")
			}
		})
	}
}
