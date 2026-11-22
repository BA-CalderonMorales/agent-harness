package agent

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/record"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// A target so small that half of it cannot even hold the lead leaves a start
// budget at or below zero. The record path must still try the minimum render
// and own the outcome, never hand an unrenderable budget to Render and fall
// back to the summarizer as if the record were empty.
func TestRecordCompactionFloorsANonPositiveStartBudget(t *testing.T) {
	loop := NewLoop(&failOnSummaryClient{t: t})
	rec, goal := minBudgetRecord()
	all := []types.Message{goal, assistantText(strings.Repeat("filler ", 400)), userText("go")}
	state := &loopState{messages: all, record: rec, toolUseContext: edgeToolContext(nil)}
	const target = 40 // target/2 minus the lead is -1

	if start := min(target/2, maxRecordSummaryTokens) - estimateText(recordSummaryLead); start > 0 {
		t.Fatalf("start budget = %d, want it at or below zero for this test", start)
	}
	if _, handled, _ := loop.compactFromRecord(state, len(all)-1, estimateTokens(all), target); !handled {
		t.Fatal("the record path gave up without trying the minimum render")
	}
}

// Each carried request is clipped to one short line.
func TestCarriedRequestsAreClippedToOneShortLine(t *testing.T) {
	loop := NewLoop(&failOnSummaryClient{t: t})
	long := "Use SQLite rather than JSON " + strings.Repeat("because of concurrent writers ", 20)
	msgs := requestsSession(userText(long), assistantText("ok"))
	state := &loopState{messages: msgs, record: record.Fold(msgs), toolUseContext: edgeToolContext(nil)}
	if _, handled, err := loop.compactFromRecord(state, len(msgs)-1, estimateTokens(msgs), 8000); err != nil || !handled {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	_, section, found := strings.Cut(firstText(state.messages), carriedRequestsTitle)
	if !found {
		t.Fatal("no carried requests section")
	}
	clipped := false
	for _, line := range strings.Split(section, "\n") {
		req, ok := strings.CutPrefix(line, "- ")
		if !ok {
			continue
		}
		if n := utf8.RuneCountInString(req); n > maxCarriedRequestRunes {
			t.Fatalf("carried request is %d runes, over %d: %q", n, maxCarriedRequestRunes, req)
		}
		if strings.HasPrefix(req, "Use SQLite rather than JSON because") {
			clipped = strings.HasSuffix(req, "…")
		}
	}
	if !clipped {
		t.Fatalf("the long request was not carried as a clipped line:\n%s", section)
	}
}
