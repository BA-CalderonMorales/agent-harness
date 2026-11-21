package agent

import (
	"testing"
	
	"github.com/BA-CalderonMorales/agent-harness/internal/core/record"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// However large the window, the record message never exceeds 600 tokens.
func TestRecordCompactionCapsTheRecordAtMaxRecordSummaryTokens(t *testing.T) {
	loop := NewLoop(&failOnSummaryClient{t: t})
	rec, msgs := fatRecord()
	if uncapped := tokensOfText(record.Render(rec, 100000)); uncapped <= maxRecordSummaryTokens+tokensOfText(recordSummaryLead) {
		t.Fatalf("fat record renders to %d tokens, too small to exercise the cap", uncapped)
	}
	all := append(append([]types.Message(nil), msgs...), assistantText("short reply"), userText("continue"))
	state := &loopState{messages: all, record: rec, toolUseContext: edgeToolContext(nil)}
	const target = 6000 // target/2 is far above the cap

	_, handled, err := loop.compactFromRecord(state, len(all)-1, estimateTokens(all), target)
	if err != nil || !handled {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	if got := summaryTokens(state.messages[0]); got > maxRecordSummaryTokens {
		t.Fatalf("record message = %d tokens, over the %d cap", got, maxRecordSummaryTokens)
	}
}
