package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools/builtin"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func TestLoopRecoversFailedDirectoryReadOnce(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(fmt.Sprintf("streaming=%v", streaming), func(t *testing.T) {
			root := t.TempDir()
			testPath := filepath.Join(root, "test_names.py")
			if err := os.WriteFile(testPath, []byte("assert normalize_name('ada') == 'Ada'"), 0600); err != nil {
				t.Fatal(err)
			}
			listing := builtin.LsRecursiveTool
			original := listing.Call
			calls := 0
			listing.Call = func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, progress tools.OnProgress) (tools.ToolResult, error) {
				calls++
				return original(input, ctx, canUse, progress)
			}
			badPath := fmt.Sprintf(`{"path":%q}`, "ls "+testPath+" 2>/dev/null || ls *.py")
			client := &scriptedLoopClient{streams: [][]types.LLMEvent{
				toolEvents("missing", "read", fmt.Sprintf(`{"file_path":%q}`, filepath.Join(root, "tests_names.py"))),
				toolEvents("bad1", "ls_recursive", badPath),
				toolEvents("bad2", "ls_recursive", badPath),
				toolEvents("fixed", "ls_recursive", fmt.Sprintf(`{"path":%q}`, root)),
				toolEvents("read", "read", fmt.Sprintf(`{"file_path":%q}`, testPath)),
				textEvents("Found test_names.py and read the actual tests."),
			}}
			loop := NewLoop(client)
			loop.Config.StreamingToolExecution = streaming
			terminal, events := runLoopForEdgeTest(t, loop, edgeParams([]tools.Tool{listing, builtin.FileReadTool}), 256)
			if terminal.Reason != TerminalReasonComplete || calls != 2 {
				t.Fatalf("reason=%s listing executions=%d, want complete with only bad1 and fixed executed", terminal.Reason, calls)
			}
			results := map[string]types.ToolResultBlock{}
			for _, event := range events {
				if message, ok := event.(types.StreamMessage); ok {
					for _, block := range message.Message.Content {
						if result, ok := block.(types.ToolResultBlock); ok {
							results[result.ToolUseID] = result
						}
					}
				}
			}
			for _, id := range []string{"missing", "bad1", "bad2"} {
				if !results[id].IsError {
					t.Fatalf("%s was not emitted as an error result: %#v", id, results[id])
				}
			}
			if !strings.Contains(results["bad2"].Content, "NOT executed") {
				t.Fatal("suppressed duplicate did not explain corrective recovery")
			}
			if results["fixed"].IsError || results["read"].IsError || !strings.Contains(fmt.Sprint(results["read"].Content), "normalize_name") {
				t.Fatal("corrected discovery/read did not succeed")
			}
			// The next provider call must see the duplicate's corrective result.
			found := false
			for _, message := range client.requestAt(3).Messages {
				for _, block := range message.Content {
					if result, ok := block.(types.ToolResultBlock); ok && result.ToolUseID == "bad2" && result.IsError {
						found = true
					}
				}
			}
			if !found {
				t.Fatal("correction was displayed but not sent back to the model")
			}
		})
	}
}

func TestLoopReadRecoveryIsBoundedAndDoesNotReplayWrites(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		for _, readOnly := range []bool{false, true} {
			t.Run(fmt.Sprintf("streaming=%v/readonly=%v", streaming, readOnly), func(t *testing.T) {
				calls := 0
				tool := edgeTool(&calls)
				tool.Capabilities.IsReadOnly = func(map[string]any) bool { return readOnly }
				tool.Call = func(map[string]any, tools.Context, tools.CanUseToolFn, tools.OnProgress) (tools.ToolResult, error) {
					calls++
					return tools.ToolResult{}, fmt.Errorf("missing path")
				}
				client := &scriptedLoopClient{streams: [][]types.LLMEvent{
					toolEvents("one", tool.Name, `{"cmd":"bad"}`),
					toolEvents("two", tool.Name, `{"cmd":"bad"}`),
					toolEvents("three", tool.Name, `{"cmd":"bad"}`),
				}}
				loop := NewLoop(client)
				loop.Config.StreamingToolExecution = streaming
				terminal, events := runLoopForEdgeTest(t, loop, edgeParams([]tools.Tool{tool}), 128)
				wantRequests := 2
				if readOnly {
					wantRequests = 3
				}
				if terminal.Reason != TerminalReasonBlockingLimit || calls != 1 || client.callCount() != wantRequests {
					t.Fatalf("reason=%s executions=%d requests=%d", terminal.Reason, calls, client.callCount())
				}
				if terminal.Error == nil || !strings.HasPrefix(terminal.Error.Error(), "tool repetition limit:") {
					t.Fatalf("missing actionable loop failure: %v", terminal.Error)
				}
				results := map[string]bool{}
				for _, event := range events {
					if message, ok := event.(types.StreamMessage); ok {
						for _, block := range message.Message.Content {
							if result, ok := block.(types.ToolResultBlock); ok {
								results[result.ToolUseID] = true
							}
						}
					}
				}
				for _, id := range []string{"one", "two", "three"}[:wantRequests] {
					if !results[id] {
						t.Fatalf("stopped turn left tool call %s without a result; continuation would be invalid", id)
					}
				}
			})
		}
	}
}
