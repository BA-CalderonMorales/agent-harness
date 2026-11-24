package agent

import (
	"context"
	"reflect"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/record"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func userText(text string) types.Message {
	return types.Message{Role: types.RoleUser, Content: []types.ContentBlock{types.TextBlock{Text: text}}}
}

func TestSeedRecordPrefersTheCallersRecord(t *testing.T) {
	given := record.Fold([]types.Message{userText("migrate settings persistence to one write path")})
	params := QueryParams{Record: given, Messages: []types.Message{userText("something else entirely")}, RecordCompaction: true}
	if got := seedRecord(params); !reflect.DeepEqual(got, given) {
		t.Fatalf("seed = %+v, want the caller's record %+v", got, given)
	}
}

func TestSeedRecordFoldsTheTranscriptWhenNoRecordIsGiven(t *testing.T) {
	msgs := []types.Message{userText("migrate settings persistence to one write path")}
	got := seedRecord(QueryParams{Messages: msgs, RecordCompaction: true})
	if want := record.Fold(msgs); !reflect.DeepEqual(got, want) {
		t.Fatalf("seed = %+v, want Fold of the messages %+v", got, want)
	}
	if got.Goal == "" {
		t.Fatal("the folded seed lost the goal")
	}
	if !seedRecord(QueryParams{}).IsZero() {
		t.Fatal("no record and no messages must seed the zero Record")
	}
}

func TestLoopStateAddIgnoresSystemMessages(t *testing.T) {
	state := &loopState{record: record.Fold([]types.Message{userText("keep the goal")})}
	before := state.record
	state.add(types.Message{Role: types.RoleSystem, Content: []types.ContentBlock{
		types.TextBlock{Text: "[Session record]\nGoal: something else\n- never push to main"},
	}})
	if !reflect.DeepEqual(state.record, before) {
		t.Fatalf("a system message changed the record: %+v -> %+v", before, state.record)
	}
	if len(state.messages) != 1 {
		t.Fatalf("messages = %d, want the system message appended", len(state.messages))
	}
}

func recordWriteTool() tools.Tool {
	return tools.NewTool(tools.Tool{
		Name: "write",
		Call: func(input map[string]any, _ tools.Context, _ tools.CanUseToolFn, _ tools.OnProgress) (tools.ToolResult, error) {
			return tools.ToolResult{Data: "wrote"}, nil
		},
		MapResult: func(result any, toolUseID string) types.ToolResultBlock {
			return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
		},
	})
}

// The loop appends the assistant's tool call and the tool's result itself.
// Its copy of the record must see both, or a later compaction renders a
// record that forgot the work done in this very query.
func TestLoopRecordFollowsMessagesAppendedMidLoop(t *testing.T) {
	client := &scriptedLoopClient{streams: [][]types.LLMEvent{
		toolEvents("call_1", "write", `{"file_path":"internal/core/state/save.go"}`),
		textEvents("done"),
	}}
	loop := NewLoop(client)
	loop.Config.AutoCompactEnabled = false
	loop.Config.DefaultMaxTurns = 5

	history := []types.Message{userText("migrate settings persistence to one write path")}
	params := edgeParams([]tools.Tool{recordWriteTool()})
	params.Messages = history
	state := edgeLoopState(history)
	state.record = record.Fold(history)

	terminal := loop.queryLoop(context.Background(), params, state, make(chan types.StreamEvent, 64))
	if terminal.Reason != TerminalReasonComplete {
		t.Fatalf("terminal = %q (%v), want complete", terminal.Reason, terminal.Error)
	}
	if len(state.record.Files) != 1 || state.record.Files[0].Path != "internal/core/state/save.go" {
		t.Fatalf("record files = %+v, want the file written mid-loop", state.record.Files)
	}
	if !state.record.Replied {
		t.Fatal("record did not see the assistant's closing reply")
	}
	if want := record.Fold(state.messages); !reflect.DeepEqual(state.record, want) {
		t.Fatalf("loop record %+v drifted from Fold of its messages %+v", state.record, want)
	}
}
