package agent

import (
	"strings"
	"testing"
	
	"github.com/BA-CalderonMorales/agent-harness/internal/core/record"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// With a small target and a huge newest message, the record fits only after
// the oldest suffix messages are dropped. Dropping must keep the newest
// message and the record, and must not fail.
func TestRecordCompactionDropsSuffixMessagesWhenTheNewestIsLarge(t *testing.T) {
	loop := NewLoop(&failOnSummaryClient{t: t})
	msgs := []types.Message{
		userText(recordGoal + " " + recordConstraint),
		assistantText("ack"),
		assistantText(strings.Repeat("b", 4*270)), // about 270 tokens
		userText("continue"),
	}
	state := &loopState{messages: msgs, record: record.Fold(msgs), toolUseContext: edgeToolContext(nil)}
	const target = 300
	keepStart := 2 // the suffix starts at the big message: big + record cannot fit
	current := estimateTokens(msgs)

	outcome, handled, err := loop.compactFromRecord(state, keepStart, current, target)
	if err != nil || !handled {
		t.Fatalf("handled=%v err=%v, want the oldest suffix message dropped and success", handled, err)
	}
	if outcome.RemovedCount != keepStart+1 {
		t.Fatalf("removed = %d, want %d (the big message dropped too)", outcome.RemovedCount, keepStart+1)
	}
	if got := estimateTokens(state.messages); got > target {
		t.Fatalf("history = %d tokens, over the %d target", got, target)
	}
	last := state.messages[len(state.messages)-1]
	if !messagesContainText([]types.Message{last}, "continue") {
		t.Fatal("the newest message was dropped")
	}
	if !messagesContainText(state.messages[:1], recordGoal) {
		t.Fatal("the record lost the goal")
	}
}
