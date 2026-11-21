package agent

import (
	"fmt"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/record"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// fatRecord folds a session that fills every capped section with long
// entries, so its render is far larger than any small budget.
func fatRecord() (record.Record, []types.Message) {
	long := strings.Repeat("segment/", 12)
	msgs := []types.Message{userText(
		"Migrate settings persistence to one write path across every package and keep behavior identical. " +
			strings.Repeat("Detail about the migration. ", 8) + "Never push directly to main. " +
			"Do not rename the public API surface of the settings package. Must keep every existing test green. " +
			"Only touch files under internal. Always run the formatter before finishing.")}
	todos := make([]any, 0, 8)
	for i := 0; i < 8; i++ {
		todos = append(todos, map[string]any{"text": fmt.Sprintf("todo %d: %s", i, strings.Repeat("plan step ", 8)), "status": "pending"})
	}
	msgs = append(msgs, assistantToolCall("tp", "todo_write", map[string]any{"todos": todos}), toolResult("tp", "ok", false))
	for i := 0; i < 12; i++ {
		id := fmt.Sprintf("fw%d", i)
		msgs = append(msgs, assistantToolCall(id, "write", map[string]any{"file_path": fmt.Sprintf("internal/%s%02d.go", long, i)}), toolResult(id, "ok", false))
	}
	for i := 0; i < 8; i++ {
		id := fmt.Sprintf("fb%d", i)
		cmd := fmt.Sprintf("go test ./internal/%s... -run TestCase%d -count=1 -v", long, i)
		msgs = append(msgs, assistantToolCall(id, "bash", map[string]any{"command": cmd}), toolResult(id, strings.Repeat("failure detail ", 10), true))
	}
	return record.Fold(msgs), msgs
}

func tokensOfText(s string) int { return (len(s) + 3) / 4 }

// When the record render at the first budget leaves the history over the
// target, the render shrinks and tries again before any message is dropped.
func TestRecordCompactionShrinksTheRenderBeforeDroppingMessages(t *testing.T) {
	loop := NewLoop(&failOnSummaryClient{t: t})
	rec, msgs := fatRecord()
	if full := record.Render(rec, 1000); tokensOfText(full) < 500 {
		t.Fatalf("fat record renders to %d tokens, too small for this test", tokensOfText(full))
	}
	// One suffix message of about 250 tokens; target 400 leaves the first
	// render budget (179) too big and a shrunk one (100) just right.
	big := assistantText(strings.Repeat("s", 4*250))
	all := append(append([]types.Message(nil), msgs...), big)
	state := &loopState{messages: all, record: rec, toolUseContext: edgeToolContext(nil)}
	const target = 400

	outcome, handled, err := loop.compactFromRecord(state, len(all)-1, estimateTokens(all), target)
	if err != nil || !handled {
		t.Fatalf("handled=%v err=%v, want success from a shrunk render", handled, err)
	}
	if outcome.RemovedCount != len(all)-1 {
		t.Fatalf("removed = %d, want the suffix message kept (%d)", outcome.RemovedCount, len(all)-1)
	}
	if got := estimateTokens(state.messages); got > target {
		t.Fatalf("history = %d tokens, over the %d target", got, target)
	}
}
