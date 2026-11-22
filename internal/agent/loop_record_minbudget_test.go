package agent

import (
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/record"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func minBudgetRecord() (record.Record, types.Message) {
	goal := userText("Migrate settings persistence to one write path. " + strings.Repeat("Keep every caller working. ", 12))
	return record.Fold([]types.Message{goal}), goal
}

func recordMessageFor(rec record.Record, tokens int) types.Message {
	return types.Message{Role: types.RoleSystem, Content: []types.ContentBlock{
		types.TextBlock{Text: recordSummaryLead + record.Render(rec, tokens)},
	}}
}

// A target so small that half of it, less the lead sentence, is under the
// minimum render budget must still try the minimum before giving up: the
// lead, a short record and a one-word message fit it.
func TestRecordCompactionTriesTheMinimumRenderWhenTheTargetIsTiny(t *testing.T) {
	loop := NewLoop(&failOnSummaryClient{t: t})
	rec, goal := minBudgetRecord()
	all := []types.Message{goal, assistantText(strings.Repeat("filler ", 400)), userText("continue")}
	state := &loopState{messages: all, record: rec, toolUseContext: edgeToolContext(nil)}
	const target = 80 // half minus the lead is 19, under the 24-token minimum

	_, handled, err := loop.compactFromRecord(state, len(all)-1, estimateTokens(all), target)
	if err != nil || !handled {
		t.Fatalf("handled=%v err=%v, want the minimum render tried and accepted", handled, err)
	}
	if got := estimateTokens(state.messages); got > target {
		t.Fatalf("history = %d tokens, over the %d target", got, target)
	}
}

// The shrink steps are 3/4 of the last budget, so the sequence can step past
// the minimum (31 then 23, never 24). Whenever the minimum render fits, the
// compaction must succeed. The sweep varies the newest message by one byte so
// some size fits at the minimum and at no larger step.
func TestRecordCompactionAcceptsAHistoryThatFitsOnlyAtTheMinimumRender(t *testing.T) {
	rec, goal := minBudgetRecord()
	const target = 400
	minOnly := 0
	for size := 1200; size <= 1500; size++ {
		newest := assistantText(strings.Repeat("n", size))
		all := []types.Message{goal, assistantText(strings.Repeat("filler ", 2000)), newest}
		fitsAtMin := estimateTokens([]types.Message{recordMessageFor(rec, minRecordSummaryTokens), newest}) <= target
		fitsAt31 := estimateTokens([]types.Message{recordMessageFor(rec, 31), newest}) <= target
		if !fitsAtMin {
			continue
		}
		if !fitsAt31 {
			minOnly++
		}
		loop := NewLoop(&failOnSummaryClient{t: t})
		state := &loopState{messages: all, record: rec, toolUseContext: edgeToolContext(nil)}
		if _, handled, err := loop.compactFromRecord(state, len(all)-1, estimateTokens(all), target); err != nil || !handled {
			t.Fatalf("newest message of %d bytes fits at the minimum render but compaction failed: handled=%v err=%v", size, handled, err)
		}
	}
	if minOnly == 0 {
		t.Fatal("the sweep never produced a size that fits only at the minimum render")
	}
}
