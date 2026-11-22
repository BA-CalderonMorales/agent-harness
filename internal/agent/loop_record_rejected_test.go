package agent

import (
	"context"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// A rejected tool call still gets a result. The record must see it: it closes
// the open call and notes the failed command.
func TestRejectedToolResultReachesTheRecord(t *testing.T) {
	state := &loopState{toolUseContext: edgeToolContext(nil)}
	use := types.ToolUseBlock{ID: "rej_1", Name: "bash", Input: map[string]any{"command": "make deploy"}}
	state.add(types.Message{Role: types.RoleAssistant, Content: []types.ContentBlock{use}})
	if len(state.record.Open) != 1 {
		t.Fatalf("open calls = %+v, want the proposal tracked", state.record.Open)
	}

	if err := emitRejectedTool(context.Background(), state, use, "The repetition guard stopped this call.", make(chan types.StreamEvent, 4)); err != nil {
		t.Fatal(err)
	}
	if len(state.record.Open) != 0 {
		t.Fatalf("open calls = %+v, want the rejection to close the call", state.record.Open)
	}
	if len(state.record.Cmds) != 1 || !state.record.Cmds[0].Failed {
		t.Fatalf("cmds = %+v, want the rejected command recorded as failed", state.record.Cmds)
	}
}
