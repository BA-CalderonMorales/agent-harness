package agent

import (
	"fmt"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/record"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// paddedRecord is a record whose body length moves in small steps with pad:
// a fixed set of long todos and commands plus twelve touched files whose
// paths grow with it. Together they exceed any budget, so the render sheds.
func paddedRecord(pad int) (record.Record, []types.Message) {
	_, msgs := fatRecord()
	msgs = msgs[:3] // the goal, constraints and the todo plan
	for i := 0; i < 12; i++ {
		id := fmt.Sprintf("pw%d", i)
		path := fmt.Sprintf("internal/%s/f%02d.go", strings.Repeat("p", pad), i)
		msgs = append(msgs, assistantToolCall(id, "write", map[string]any{"file_path": path}), toolResult(id, "ok", false))
	}
	return record.Fold(msgs), msgs
}

// The 600-token cap covers the whole record message, lead sentence included.
// The scan finds a record whose body lands just under the body-only cap, the
// case where the lead pushes the message past 600.
func TestRecordMessageIncludingTheLeadStaysWithinTheCap(t *testing.T) {

	lowBytes := maxRecordSummaryTokens*4 - len(recordSummaryLead)
	highBytes := maxRecordSummaryTokens * 4
	for pad := 1; pad <= 110; pad++ {
		rec, msgs := paddedRecord(pad)
		body := record.Render(rec, maxRecordSummaryTokens)
		if len(body) <= lowBytes || len(body) > highBytes {
			continue
		}
		loop := NewLoop(&failOnSummaryClient{t: t})
		all := append(append([]types.Message(nil), msgs...), assistantText(strings.Repeat("filler ", 3000)), assistantText("short reply"), userText("continue"))
		state := &loopState{messages: all, record: rec, toolUseContext: edgeToolContext(nil)}
		if _, handled, err := loop.compactFromRecord(state, len(all)-2, estimateTokens(all), 6000); err != nil || !handled {
			t.Fatalf("pad %d: handled=%v err=%v", pad, handled, err)
		}
		if got := summaryTokens(state.messages[0]); got > maxRecordSummaryTokens {
			t.Fatalf("pad %d: record message = %d tokens, over the %d cap (body %d bytes)", pad, got, maxRecordSummaryTokens, len(body))
		}
		return
	}
	t.Fatal("no padded record landed a body between the body-only cap and the full cap")
}
