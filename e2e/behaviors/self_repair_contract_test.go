package behaviors

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/permissions"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/internal/testharness"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func TestBehavior_BoundedRepairUsesDistinctToolsAndAdaptsAfterFailure(t *testing.T) {
	f := testharness.NewFixture(t)
	target := filepath.Join(f.WorkDir, "target.txt")
	if err := os.WriteFile(target, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	f.SetPermissionMode(permissions.ModeDontAsk)
	f.SetAlwaysAllow("write")
	f.SetMockResponses(
		toolResponseAt(f.WorkDir,
			[2]string{"bad-1", "missing"}, [2]string{"glob-1", "glob"},
			[2]string{"grep-1", "grep"}, [2]string{"read-1", "read"},
			[2]string{"ls-1", "ls"},
		),
		toolResponseAt(f.WorkDir, [2]string{"write-1", "write"}),
		toolResponseAt(f.WorkDir, [2]string{"read-2", "read"}),
		llm.MockTextResponse("verified: repaired"),
	)

	events := f.QueryLoop(nil, "Repair target.txt and verify the result.")
	if terminal := lastTerminal(events); terminal == nil || terminal.Reason != "complete" {
		t.Fatalf("terminal = %#v, want complete", terminal)
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != "repaired" {
		t.Fatalf("target = %q, err=%v; want repaired; requests=%d events=%#v", got, err, len(f.MockRequests), events)
	}
	if len(f.MockRequests) != 4 {
		t.Fatalf("provider requests = %d, want 4", len(f.MockRequests))
	}
	if !requestHasErrorResult(f.MockRequests[1]) {
		t.Fatal("adaptation request did not receive the failed tool result")
	}
	if countToolUses(events) != 7 {
		t.Fatalf("observable tool uses = %d, want 7", countToolUses(events))
	}
}

func TestBehavior_FixtureRestoresHomeEnvironment(t *testing.T) {
	want, wantSet := os.LookupEnv("HOME")
	t.Run("fixture", func(t *testing.T) {
		f := testharness.NewFixture(t)
		if got := os.Getenv("HOME"); got != f.WorkDir {
			t.Fatalf("fixture HOME = %q, want %q", got, f.WorkDir)
		}
	})
	got, gotSet := os.LookupEnv("HOME")
	if gotSet != wantSet || got != want {
		t.Fatalf("HOME leaked after fixture: got (%q, set=%t), want (%q, set=%t)", got, gotSet, want, wantSet)
	}
}

func TestBehavior_DenialIsObservableAndSideEffectFree(t *testing.T) {
	f := testharness.NewFixture(t)
	f.SetAlwaysDeny("write")
	target := filepath.Join(f.WorkDir, "denied.txt")
	f.SetMockResponses(toolResponseAt(f.WorkDir, [2]string{"deny-1", "write"}), llm.MockTextResponse("I cannot write that"))

	events := f.QueryLoop(nil, "Write the file.")
	if !hasErrorToolResult(events, "deny-1") {
		t.Fatal("denied tool did not produce an error ToolResultBlock")
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("denied write changed filesystem: stat err=%v", err)
	}
	if terminal := lastTerminal(events); terminal == nil || terminal.Reason != "complete" {
		t.Fatalf("terminal = %#v, want complete after honest denial", terminal)
	}
}

func TestBehavior_CancellationAndBoundedTerminals(t *testing.T) {
	t.Run("cancellation", func(t *testing.T) {
		f := testharness.NewFixture(t)
		f.SetPermissionMode(permissions.ModeDontAsk)
		f.SetAlwaysAllow("bash")
		f.ToolRegistry.RegisterBuiltIn(tools.NewTool(tools.Tool{
			Name: "wait_cancel",
			CheckPermissions: func(map[string]any, tools.Context) tools.PermissionDecision {
				return tools.PermissionDecision{Behavior: tools.Allow}
			},
			Call: func(input map[string]any, ctx tools.Context, _ tools.CanUseToolFn, _ tools.OnProgress) (tools.ToolResult, error) {
				<-ctx.AbortController.Done()
				return tools.ToolResult{}, ctx.AbortController.Err()
			},
			MapResult: func(result any, id string) types.ToolResultBlock {
				return types.ToolResultBlock{ToolUseID: id, Content: "cancelled"}
			},
		}))
		f.SetMockResponses(toolResponse([2]string{"slow-1", "wait_cancel"}))
		ctx, cancel := context.WithCancel(context.Background())
		result := make(chan []types.StreamEvent, 1)
		go func() { result <- f.QueryLoopContext(ctx, nil, "wait") }()
		time.Sleep(20 * time.Millisecond)
		cancel()
		select {
		case events := <-result:
			if terminal := lastTerminal(events); terminal == nil ||
				(terminal.Reason != "user_interrupt" &&
					(terminal.Reason != "error" || !errors.Is(terminal.Error, context.Canceled))) {
				t.Fatalf("terminal = %#v, err=%v, want an honest cancellation terminal", terminal, terminal.Error)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("cancelled query did not converge")
		}
	})

	t.Run("budget", func(t *testing.T) {
		f := testharness.NewFixture(t)
		f.SetPermissionMode(permissions.ModeDontAsk)
		f.Loop.Config.MaxToolCalls = 1
		f.SetMockResponses(toolResponse([2]string{"one", "glob"}, [2]string{"two", "ls"}))
		events := f.QueryLoop(nil, "inspect")
		if terminal := lastTerminal(events); terminal == nil || terminal.Reason != "blocking_limit" {
			t.Fatalf("terminal = %#v, want blocking_limit", terminal)
		}
	})

	t.Run("convergence", func(t *testing.T) {
		f := testharness.NewFixture(t)
		f.SetPermissionMode(permissions.ModeDontAsk)
		f.SetMockResponses(toolResponse([2]string{"same-1", "glob"}), toolResponse([2]string{"same-2", "glob"}))
		events := f.QueryLoop(nil, "inspect")
		if terminal := lastTerminal(events); terminal == nil || terminal.Reason != "blocking_limit" {
			t.Fatalf("terminal = %#v, want blocking_limit", terminal)
		}
	})
}

func toolResponse(calls ...[2]string) []types.LLMEvent {
	return toolResponseAt(".", calls...)
}

func toolResponseAt(root string, calls ...[2]string) []types.LLMEvent {
	events := []types.LLMEvent{types.LLMMessageStart{ID: "msg"}}
	for _, call := range calls {
		input := map[string]any{}
		switch call[1] {
		case "glob":
			input = map[string]any{"pattern": "*.txt", "path": root}
		case "grep":
			input = map[string]any{"pattern": "old", "path": root}
		case "read":
			input = map[string]any{"file_path": filepath.Join(root, "target.txt")}
			if call[0] == "read-2" {
				input["offset"] = 0
			}
		case "ls":
			input = map[string]any{"path": root}
		case "bash":
			input = map[string]any{"command": "sleep 5"}
		case "write":
			input = map[string]any{"file_path": filepath.Join(root, "target.txt"), "content": "repaired"}
		}
		encoded, _ := json.Marshal(input)
		events = append(events, types.LLMToolUseDelta{ID: call[0], Name: call[1], Delta: string(encoded)})
	}
	events = append(events, types.LLMMessageStop{StopReason: "tool_use"})
	return events
}

func lastTerminal(events []types.StreamEvent) *types.StreamTerminal {
	for i := len(events) - 1; i >= 0; i-- {
		if terminal, ok := events[i].(types.StreamTerminal); ok {
			return &terminal
		}
	}
	return nil
}

func countToolUses(events []types.StreamEvent) int {
	count := 0
	for _, event := range events {
		if message, ok := event.(types.StreamMessage); ok {
			for _, block := range message.Message.Content {
				if _, ok := block.(types.ToolUseBlock); ok {
					count++
				}
			}
		}
	}
	return count
}

func hasErrorToolResult(events []types.StreamEvent, id string) bool {
	for _, event := range events {
		message, ok := event.(types.StreamMessage)
		if !ok {
			continue
		}
		for _, block := range message.Message.Content {
			if result, ok := block.(types.ToolResultBlock); ok && result.ToolUseID == id && result.IsError && strings.Contains(strings.ToLower(result.Content), "permission") {
				return true
			}
		}
	}
	return false
}

func requestHasErrorResult(req llm.Request) bool {
	for _, message := range req.Messages {
		for _, block := range message.Content {
			if result, ok := block.(types.ToolResultBlock); ok && result.IsError {
				return true
			}
		}
	}
	return false
}
