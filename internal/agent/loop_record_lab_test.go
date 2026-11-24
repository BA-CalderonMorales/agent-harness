package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/record"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// recordLabParams is a 4096-window query over recordSession with the record
// compaction lab set as given.
func recordLabParams(history []types.Message, enabled bool) QueryParams {
	params := edgeParams(nil)
	params.Messages = history
	params.SystemPrompt = strings.Repeat("s", 2800*4)
	params.PromptTokens = 2800
	params.RecordCompaction = enabled
	return params
}

func recordLabTerminal(t *testing.T, events <-chan types.StreamEvent) types.StreamTerminal {
	t.Helper()
	var terminal *types.StreamTerminal
	for ev := range events {
		if tm, ok := ev.(types.StreamTerminal); ok {
			terminal = &tm
		}
	}
	if terminal == nil {
		t.Fatal("the query ended without a terminal event")
	}
	return *terminal
}

// With the record compaction lab off, a long tool-heavy session is shortened
// the pre-record way: the model writes the summary, and the record is unused.
func TestRecordLabOffCompactsWithTheModelSummary(t *testing.T) {
	windowed := &windowedClient{window: 4096}
	loop := NewLoop(windowed)
	loop.Config.BlockingTokenLimit = 4096
	loop.Config.DefaultMaxTurns = 1

	events, err := loop.Query(context.Background(), recordLabParams(recordSession(4096), false))
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if terminal := recordLabTerminal(t, events); terminal.Reason != string(TerminalReasonComplete) {
		t.Fatalf("terminal = %q (%v), want complete", terminal.Reason, terminal.Error)
	}
	requests, refused := windowed.snapshot()
	if refused != 0 {
		t.Fatalf("the server refused %d request(s)", refused)
	}
	summaries, main := splitWindowedRequests(requests)
	if len(summaries) == 0 {
		t.Fatal("summary requests = 0; with the lab off the model must write the summary")
	}
	for _, req := range main {
		if messagesContainText(req.Messages, "this record replaces them") {
			t.Fatal("a request carried the session record message with the lab off")
		}
	}
}

// With the lab on, the same history compacts from the record: no summary
// request reaches the model and the turn completes with the goal in view.
func TestRecordLabOnCompactsWithoutAModelSummary(t *testing.T) {
	windowed := &windowedClient{window: 4096}
	client := &noSummaryWindowClient{windowedClient: windowed, t: t}
	loop := NewLoop(client)
	loop.Config.BlockingTokenLimit = 4096
	loop.Config.DefaultMaxTurns = 1

	events, err := loop.Query(context.Background(), recordLabParams(recordSession(4096), true))
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if terminal := recordLabTerminal(t, events); terminal.Reason != string(TerminalReasonComplete) {
		t.Fatalf("terminal = %q (%v), want complete", terminal.Reason, terminal.Error)
	}
	requests, refused := windowed.snapshot()
	if refused != 0 {
		t.Fatalf("the server refused %d request(s)", refused)
	}
	if summaries, main := splitWindowedRequests(requests); len(summaries) != 0 || len(main) != 1 {
		t.Fatalf("summaries = %d, main = %d; want 0 and 1", len(summaries), len(main))
	}
	if !messagesContainText(requests[0].Messages, recordGoal) {
		t.Fatal("the request lost the goal")
	}
}

// With the lab off, a record the caller hands over is not seeded and the loop
// does not apply messages to one: no record work at all.
func TestRecordLabOffKeepsTheRecordOutOfTheLoop(t *testing.T) {
	msgs := []types.Message{userText("migrate settings persistence to one write path")}
	params := QueryParams{Record: record.Fold(msgs), Messages: msgs}
	if got := seedRecord(params); !got.IsZero() {
		t.Fatalf("seed with the lab off = %+v, want the zero Record", got)
	}
	state := &loopState{recordOff: true}
	state.add(userText("keep going"), assistantText("ok"))
	if !state.record.IsZero() {
		t.Fatalf("loop record with the lab off = %+v, want zero", state.record)
	}
	if len(state.messages) != 2 {
		t.Fatalf("messages = %d, want both appended", len(state.messages))
	}
}
