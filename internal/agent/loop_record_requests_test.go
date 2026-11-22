package agent

import (
	"fmt"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/record"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func requestsSession(extra ...types.Message) []types.Message {
	msgs := []types.Message{
		userText("Migrate settings persistence to one write path."),
		assistantText("Understood."),
		userText("Use SQLite rather than JSON for the store."),
		assistantText("Switching the store to SQLite."),
		assistantToolCall("rq1", "tool_result_only_probe", map[string]any{}),
		toolResult("rq1", "TOOL-OUTPUT-NOT-A-REQUEST", false),
		assistantText(strings.Repeat("progress ", 2000)),
	}
	msgs = append(msgs, extra...)
	return append(msgs, userText("continue"))
}

// A later instruction is not the goal and not a constraint sentence, so the
// record alone drops it. Compaction must carry the removed user requests.
func TestRecordCompactionCarriesLaterUserRequests(t *testing.T) {
	loop := NewLoop(&failOnSummaryClient{t: t})
	msgs := requestsSession()
	state := &loopState{messages: msgs, record: record.Fold(msgs), toolUseContext: edgeToolContext(nil)}

	_, handled, err := loop.compactFromRecord(state, len(msgs)-1, estimateTokens(msgs), 1000)
	if err != nil || !handled {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	text := firstText(state.messages)
	if !strings.Contains(text, "Use SQLite rather than JSON") {
		t.Fatalf("the later requirement was lost:\n%s", text)
	}
	if strings.Contains(text, "TOOL-OUTPUT-NOT-A-REQUEST") {
		t.Fatal("tool output was carried as a user request")
	}
	if strings.Count(text, "Migrate settings persistence") != 1 {
		t.Fatalf("the goal must appear once, as the goal:\n%s", text)
	}
}

// The carried requests are bounded: only the newest few, each clipped, all
// inside the record message's own token budget.
func TestCarriedUserRequestsAreBounded(t *testing.T) {
	loop := NewLoop(&failOnSummaryClient{t: t})
	var extra []types.Message
	for i := 0; i < 20; i++ {
		extra = append(extra, userText(fmt.Sprintf("Request %02d: %s", i, strings.Repeat("detail ", 60))), assistantText("ok"))
	}
	msgs := requestsSession(extra...)
	state := &loopState{messages: msgs, record: record.Fold(msgs), toolUseContext: edgeToolContext(nil)}
	const target = 1000

	if _, handled, err := loop.compactFromRecord(state, len(msgs)-1, estimateTokens(msgs), target); err != nil || !handled {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	text := firstText(state.messages)
	if !strings.Contains(text, "Request 19") || strings.Contains(text, "Request 00") {
		t.Fatalf("want the newest requests and not the oldest:\n%s", text)
	}
	if got := summaryTokens(state.messages[0]); got > target/2 {
		t.Fatalf("record message = %d tokens, over its %d budget", got, target/2)
	}
	if got := estimateTokens(state.messages); got > target {
		t.Fatalf("history = %d tokens, over the %d target", got, target)
	}
}

// With room to spare, exactly the newest maxCarriedRequests are carried.
func TestCarriedUserRequestsStopAtTheCountBound(t *testing.T) {
	loop := NewLoop(&failOnSummaryClient{t: t})
	var extra []types.Message
	for i := 0; i < 20; i++ {
		extra = append(extra, userText(fmt.Sprintf("Short request %02d", i)), assistantText("ok"))
	}
	msgs := requestsSession(extra...)
	state := &loopState{messages: msgs, record: record.Fold(msgs), toolUseContext: edgeToolContext(nil)}
	if _, handled, err := loop.compactFromRecord(state, len(msgs)-1, estimateTokens(msgs), 8000); err != nil || !handled {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	if got := strings.Count(firstText(state.messages), "Short request"); got != maxCarriedRequests {
		t.Fatalf("carried %d requests, want %d", got, maxCarriedRequests)
	}
}

// A fat record plus carried requests still fits one budget: the requests take
// their share out of the record's room, not on top of it.
func TestCarriedUserRequestsShareTheRecordBudget(t *testing.T) {
	for _, target := range []int{1000, 6000} {
		loop := NewLoop(&failOnSummaryClient{t: t})
		_, fat := fatRecord()
		later := userText("Use SQLite rather than JSON for the store. " + strings.Repeat("why ", 20))
		msgs := append(append([]types.Message(nil), fat...), later, assistantText(strings.Repeat("progress ", 2000)), userText("continue"))
		state := &loopState{messages: msgs, record: record.Fold(msgs), toolUseContext: edgeToolContext(nil)}
		if _, handled, err := loop.compactFromRecord(state, len(msgs)-1, estimateTokens(msgs), target); err != nil || !handled {
			t.Fatalf("target %d: handled=%v err=%v", target, handled, err)
		}
		if got, limit := summaryTokens(state.messages[0]), min(target/2, maxRecordSummaryTokens); got > limit {
			t.Fatalf("target %d: record message = %d tokens, over its %d budget", target, got, limit)
		}
		if !strings.Contains(firstText(state.messages), "Use SQLite") {
			t.Fatalf("target %d: the later request was lost", target)
		}
	}
}

// A carried request lives in the record message. The next compaction removes
// that message, so it must carry the request forward again.
func TestCarriedUserRequestsSurviveASecondCompaction(t *testing.T) {
	loop := NewLoop(&failOnSummaryClient{t: t})
	msgs := requestsSession()
	state := &loopState{messages: msgs, record: record.Fold(msgs), toolUseContext: edgeToolContext(nil)}
	const target = 1000
	if _, handled, err := loop.compactFromRecord(state, len(msgs)-1, estimateTokens(msgs), target); err != nil || !handled {
		t.Fatalf("first compaction: handled=%v err=%v", handled, err)
	}

	state.add(assistantText(strings.Repeat("more progress ", 1500)))
	state.add(userText("next"))
	if _, handled, err := loop.compactFromRecord(state, len(state.messages)-1, estimateTokens(state.messages), target); err != nil || !handled {
		t.Fatalf("second compaction: handled=%v err=%v", handled, err)
	}
	text := firstText(state.messages)
	if !strings.Contains(text, "Use SQLite rather than JSON") {
		t.Fatalf("the requirement did not survive the second compaction:\n%s", text)
	}
	if !strings.Contains(text, "- continue") {
		t.Fatalf("the user turn removed by the second compaction was dropped:\n%s", text)
	}
	if strings.Count(text, carriedRequestsTitle) != 1 {
		t.Fatalf("want one requests section, not a nested copy:\n%s", text)
	}
}

// Only the goal itself is skipped. After an earlier compaction the first user
// message in the removed range is an ordinary request and must be carried.
func TestTheFirstRemovedUserMessageIsCarriedWhenItIsNotTheGoal(t *testing.T) {
	loop := NewLoop(&failOnSummaryClient{t: t})
	rec := record.Fold([]types.Message{userText("Migrate settings persistence to one write path.")})
	msgs := []types.Message{
		userText("Keep the SQLite store behind the interface."),
		assistantText(strings.Repeat("progress ", 2000)),
		userText("go"),
	}
	state := &loopState{messages: msgs, record: rec, toolUseContext: edgeToolContext(nil)}
	if _, handled, err := loop.compactFromRecord(state, len(msgs)-1, estimateTokens(msgs), 1000); err != nil || !handled {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	if text := firstText(state.messages); !strings.Contains(text, "Keep the SQLite store behind the interface.") {
		t.Fatalf("a non-goal first message was dropped as the goal:\n%s", text)
	}
}
