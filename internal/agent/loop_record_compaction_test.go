package agent

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/record"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

const (
	recordGoal       = "Migrate settings persistence to one write path."
	recordConstraint = "Never push directly to main."
	recordPending    = "add the regression test"
	recordLastFile   = "internal/core/state/writer_last.go"
)

func assistantToolCall(id, name string, input map[string]any) types.Message {
	return types.Message{Role: types.RoleAssistant, Content: []types.ContentBlock{
		types.ToolUseBlock{ID: id, Name: name, Input: input},
	}}
}

func toolResult(id, content string, isError bool) types.Message {
	return types.Message{Role: types.RoleUser, Content: []types.ContentBlock{
		types.ToolResultBlock{ToolUseID: id, Content: content, IsError: isError},
	}}
}

func assistantText(text string) types.Message {
	return types.Message{Role: types.RoleAssistant, Content: []types.ContentBlock{types.TextBlock{Text: text}}}
}

// recordSession is a long tool-using session: a goal, a constraint, a todo
// plan with one pending item, a dozen files touched, a failing test, then
// filler until it is about three windows long, ending on a short user turn.
func recordSession(window int) []types.Message {
	msgs := []types.Message{
		userText(recordGoal + " " + recordConstraint),
		assistantToolCall("t1", "todo_write", map[string]any{"todos": []any{
			map[string]any{"text": "unify the writers", "status": "done"},
			map[string]any{"text": recordPending, "status": "pending"},
		}}),
		toolResult("t1", "ok", false),
	}
	for i := 0; i < 12; i++ {
		path := fmt.Sprintf("internal/core/state/writer_%02d.go", i)
		if i == 11 {
			path = recordLastFile
		}
		id := fmt.Sprintf("w%d", i)
		msgs = append(msgs, assistantToolCall(id, "write", map[string]any{"file_path": path}), toolResult(id, "wrote", false))
	}
	for i := 0; i < 8; i++ {
		id := fmt.Sprintf("b%d", i)
		cmd := fmt.Sprintf("ls -la internal/core/state/writer_%02d.go", i)
		if i == 7 {
			cmd = "go test ./internal/core/state/... -count=1"
		}
		msgs = append(msgs, assistantToolCall(id, "bash", map[string]any{"command": cmd}),
			toolResult(id, "FAIL: settings writer still races on save", i == 7))
	}
	filler := strings.Repeat("progress ", 45)
	for estimateTokens(msgs) < 3*window {
		msgs = append(msgs, userText("next"), assistantText(filler))
	}
	return append(msgs, userText("continue"))
}

// failOnSummaryClient fails the test if anything asks the model to summarize.
type failOnSummaryClient struct {
	t        *testing.T
	requests atomic.Int32
	answer   string
}

func (c *failOnSummaryClient) Stream(_ context.Context, req llm.Request) (<-chan types.LLMEvent, error) {
	c.requests.Add(1)
	if strings.Contains(req.SystemPrompt, "context summarizer") {
		c.t.Errorf("a summarizer request reached the model; record compaction must not call it")
	}
	events := textEvents(c.answer)
	out := make(chan types.LLMEvent, len(events))
	for _, e := range events {
		out <- e
	}
	close(out)
	return out, nil
}

func summaryTokens(msg types.Message) int {
	return estimateTokens([]types.Message{msg})
}

// At every window the record replaces the removed turns with no model call,
// keeps the goal, the pending todo, a constraint and the newest touched file,
// and the result fits the target. The fixed prompt scales with the window
// (2800 at 4096, the measured Lemonade cost).
func TestRecordCompactionFitsEveryWindowWithoutAModelCall(t *testing.T) {
	for _, tc := range []struct{ window, prompt int }{{1024, 700}, {2048, 1400}, {4096, 2800}, {8192, 5600}} {
		t.Run(fmt.Sprintf("window_%d", tc.window), func(t *testing.T) {
			client := &failOnSummaryClient{t: t}
			loop := NewLoop(client)
			loop.Config.BlockingTokenLimit = tc.window
			history := recordSession(tc.window)
			state := &loopState{
				messages:       history,
				promptTokens:   tc.prompt,
				record:         record.Fold(history),
				toolUseContext: edgeToolContext(nil),
			}
			target := (tc.window - tc.prompt) * 8 / 10

			outcome, err := loop.compactMessages(context.Background(), state, false)
			if err != nil {
				t.Fatalf("compactMessages: %v", err)
			}
			if got := client.requests.Load(); got != 0 {
				t.Fatalf("model requests = %d, want 0", got)
			}
			if !outcome.changed() || outcome.AfterTokens >= outcome.BeforeTokens {
				t.Fatalf("outcome = %+v, want a smaller history", outcome)
			}
			if got := estimateTokens(state.messages); got > target {
				t.Fatalf("compacted history = %d tokens, over the %d target", got, target)
			}
			if got := estimateTokens(state.messages) + tc.prompt; got > tc.window {
				t.Fatalf("history plus prompt = %d tokens, over the %d window", got, tc.window)
			}
			head := state.messages[0]
			if head.Role != types.RoleSystem {
				t.Fatalf("first message role = %q, want the record as a system message", head.Role)
			}
			if limit := min(600, target/2); summaryTokens(head) > limit {
				t.Fatalf("record message = %d tokens, over its %d budget", summaryTokens(head), limit)
			}
			text := firstText(state.messages)
			for _, want := range []string{"replaces", "Goal: " + recordGoal[:20], recordPending, "Never push", recordLastFile} {
				if !strings.Contains(text, want) {
					t.Errorf("record message lost %q:\n%s", want, text)
				}
			}
			if last := state.messages[len(state.messages)-1]; !messagesContainText([]types.Message{last}, "continue") {
				t.Fatal("the newest message did not survive compaction")
			}
		})
	}
}

// A record with no goal and no work does not replace the turns; the model
// writes the summary as before.
func TestEmptyRecordFallsBackToTheModelSummary(t *testing.T) {
	for name, rec := range map[string]record.Record{
		"zero":    {},
		"replied": record.Apply(record.Record{}, assistantText("hello")),
	} {
		t.Run(name, func(t *testing.T) {
			client := &windowedClient{window: 4096}
			loop := NewLoop(client)
			loop.Config.BlockingTokenLimit = 4096
			state := &loopState{
				messages:       windowHistory(120),
				promptTokens:   2800,
				record:         rec,
				toolUseContext: edgeToolContext(nil),
			}
			if _, err := loop.compactMessages(context.Background(), state, false); err != nil {
				t.Fatalf("compactMessages: %v", err)
			}
			requests, _ := client.snapshot()
			if summaries, _ := splitWindowedRequests(requests); len(summaries) == 0 {
				t.Fatal("no summarizer request; an empty record must fall back to the model")
			}
		})
	}
}

// One message larger than the whole target cannot be saved by any record.
// That is an error, never a silent oversize history.
func TestRecordCompactionErrorsWhenTheNewestMessageAloneOverflows(t *testing.T) {
	loop := NewLoop(&failOnSummaryClient{t: t})
	loop.Config.BlockingTokenLimit = 1024
	history := append(recordSession(1024), assistantText(strings.Repeat("huge ", 1000)))
	state := &loopState{messages: history, promptTokens: 700, record: record.Fold(history), toolUseContext: edgeToolContext(nil)}
	if _, err := loop.compactMessages(context.Background(), state, false); err == nil {
		t.Fatal("want an error when the newest message alone exceeds the target")
	}
}

// The whole loop at the measured Lemonade shape: a 4096 window, 2800 tokens
// of system prompt and tools, a session of three windows. The turn completes,
// the server never refuses, no summarizer runs, and the model sees the goal.
func TestQueryLoopCompactsFromTheRecordInA4096Window(t *testing.T) {
	windowed := &windowedClient{window: 4096}
	client := &noSummaryWindowClient{windowedClient: windowed, t: t}
	loop := NewLoop(client)
	loop.Config.BlockingTokenLimit = 4096
	loop.Config.DefaultMaxTurns = 1

	history := recordSession(4096)
	params := edgeParams(nil)
	params.Messages = history
	params.SystemPrompt = strings.Repeat("s", 2800*4)
	params.PromptTokens = 2800
	state := edgeLoopState(history)
	state.promptTokens = 2800
	state.record = record.Fold(history)

	terminal := loop.queryLoop(context.Background(), params, state, make(chan types.StreamEvent, 512))
	if terminal.Reason != TerminalReasonComplete {
		t.Fatalf("terminal = %q (%v), want complete", terminal.Reason, terminal.Error)
	}
	requests, refused := windowed.snapshot()
	if refused != 0 {
		t.Fatalf("the server refused %d request(s)", refused)
	}
	if len(requests) != 1 {
		t.Fatalf("requests = %d, want exactly the one main request", len(requests))
	}
	if !messagesContainText(requests[0].Messages, recordGoal) {
		t.Fatal("the first request lost the goal")
	}
}

type noSummaryWindowClient struct {
	*windowedClient
	t *testing.T
}

func (c *noSummaryWindowClient) Stream(ctx context.Context, req llm.Request) (<-chan types.LLMEvent, error) {
	if strings.Contains(req.SystemPrompt, "context summarizer") {
		c.t.Errorf("a summarizer request reached the model")
	}
	return c.windowedClient.Stream(ctx, req)
}

// A resumed oversized session compacts on its first query from the record
// that the query folds out of the transcript: zero summarizer requests.
func TestResumedOversizedSessionCompactsFromTheFoldedRecord(t *testing.T) {
	windowed := &windowedClient{window: 4096}
	client := &noSummaryWindowClient{windowedClient: windowed, t: t}
	loop := NewLoop(client)
	loop.Config.BlockingTokenLimit = 4096
	loop.Config.DefaultMaxTurns = 1

	history := recordSession(4096)
	params := edgeParams(nil)
	params.Messages = history
	params.SystemPrompt = strings.Repeat("s", 2800*4)
	params.PromptTokens = 2800     // no params.Record: a resume with only the transcript
	params.RecordCompaction = true // the lab is on; the record is folded from the transcript

	events, err := loop.Query(context.Background(), params)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	var terminal *types.StreamTerminal
	for ev := range events {
		if tm, ok := ev.(types.StreamTerminal); ok {
			terminal = &tm
		}
	}
	if terminal == nil || terminal.Reason != string(TerminalReasonComplete) {
		t.Fatalf("terminal = %+v, want complete", terminal)
	}
	requests, refused := windowed.snapshot()
	if refused != 0 || len(requests) != 1 {
		t.Fatalf("refused = %d, requests = %d, want 0 and 1", refused, len(requests))
	}
	if !messagesContainText(requests[0].Messages, recordGoal) {
		t.Fatal("the resumed turn's request lost the goal")
	}
}
