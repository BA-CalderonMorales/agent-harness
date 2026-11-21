package agent

import (
	"context"
	"testing"
	
	"github.com/BA-CalderonMorales/agent-harness/internal/core/record"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// Batch tool execution appends its results itself. The record must see them,
// or the files a batch wrote are missing from every later compaction.
func TestLoopRecordSeesBatchExecutedToolResults(t *testing.T) {
	client := &scriptedLoopClient{streams: [][]types.LLMEvent{
		toolEvents("batch_1", "write", `{"file_path":"internal/core/state/batch.go"}`),
		textEvents("done"),
	}}
	loop := NewLoop(client)
	loop.Config.AutoCompactEnabled = false
	loop.Config.StreamingToolExecution = false
	loop.Config.DefaultMaxTurns = 5

	history := []types.Message{userText(recordGoal)}
	params := edgeParams([]tools.Tool{recordWriteTool()})
	params.Messages = history
	state := edgeLoopState(history)
	state.record = record.Fold(history)

	terminal := loop.queryLoop(context.Background(), params, state, make(chan types.StreamEvent, 64))
	if terminal.Reason != TerminalReasonComplete {
		t.Fatalf("terminal = %q (%v)", terminal.Reason, terminal.Error)
	}
	if len(state.record.Files) != 1 || state.record.Files[0].Path != "internal/core/state/batch.go" {
		t.Fatalf("record files = %+v, want the batch-written file", state.record.Files)
	}
	if len(state.record.Open) != 0 {
		t.Fatalf("open calls = %+v, want the batch result to close the call", state.record.Open)
	}
}
