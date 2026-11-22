package agent

import (
	"fmt"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/diag"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/record"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"github.com/google/uuid"
)

const (
	// maxRecordSummaryTokens caps the record message however large the
	// window is. The record's own caps keep it near this size anyway.
	maxRecordSummaryTokens = 600

	// minRecordSummaryTokens is where shrinking the render stops and the
	// recent suffix starts losing messages instead.
	minRecordSummaryTokens = 24

	recordSummaryLead = "Earlier turns were removed to fit the context window; this record replaces them.\n"
)

// hasRecordContent reports whether the record holds something worth
// standing in for the removed turns: a goal, or work done or planned. A
// record that only knows a reply was sent does not.
func hasRecordContent(r record.Record) bool {
	return r.Goal != "" || len(r.Files) > 0 || len(r.Cmds) > 0 || len(r.Todos) > 0
}

// compactFromRecord replaces the history before keepStart with the loop's
// session record and makes no model call. It reports handled=false when the
// record is too thin to stand in, so the caller falls back to a model-written
// summary.
//
// The record render gets min(600, target/2) estimated tokens. If the record
// plus the recent suffix still misses the target, the render shrinks first,
// then the oldest suffix messages go, and only when one message and a
// minimal record still do not fit does it return an error.
func (l *Loop) compactFromRecord(state *loopState, keepStart, current, target int) (compactionOutcome, bool, error) {
	if !hasRecordContent(state.record) {
		return compactionOutcome{}, false, nil
	}
	// The cap covers the whole record message, so the lead comes out of it.
	budget := min(target/2, maxRecordSummaryTokens) - estimateText(recordSummaryLead)
	last := len(state.messages) - 1
	var after int
	for start := keepStart; start <= last; start++ {
		suffix := state.messages[start:]
		reqs := earlierUserRequests(state.messages[:start], state.record.Goal)
		for _, tokens := range renderBudgets(budget) {
			body := recordBody(state.record, reqs, tokens)
			if body == "" {
				return compactionOutcome{}, false, nil
			}
			compacted := make([]types.Message, 0, 1+len(suffix))
			compacted = append(compacted, types.Message{
				UUID:      uuid.New().String(),
				Role:      types.RoleSystem,
				Timestamp: time.Now(),
				Content:   []types.ContentBlock{types.TextBlock{Text: recordSummaryLead + body}},
			})
			compacted = append(compacted, suffix...)
			after = estimateTokens(compacted)
			if after > target || after >= current {
				continue
			}
			removed := start
			state.messages = compacted
			diag.Info("context.compacted", fmt.Sprintf(
				"record replaced %d messages · tokens %d → %d · render budget %d",
				removed, current, after, tokens,
			))
			return compactionOutcome{
				Messages:     append([]types.Message(nil), compacted...),
				RemovedCount: removed,
				BeforeTokens: current,
				AfterTokens:  after,
				Notice: fmt.Sprintf(
					"[Context compacted: replaced %d older messages with the session record, %d estimated tokens → %d]",
					removed, current, after,
				),
			}, true, nil
		}
	}
	return compactionOutcome{}, true, fmt.Errorf(
		"session record and the newest message exceed the bounded target (%d estimated tokens, target %d)",
		after, target,
	)
}

// renderBudgets lists the render budgets to try, largest first. They step
// down by a quarter and always include the minimum, which a 3/4 progression
// can step past (31 then 23, never 24). A start below the minimum, as a tiny
// target leaves, tries the minimum first, because the whole-message check
// against the target decides whether it fits, and then the start itself.
func renderBudgets(start int) []int {
	var out []int
	for b := max(start, minRecordSummaryTokens); ; {
		out = append(out, b)
		if b <= minRecordSummaryTokens {
			break
		}
		b = max(b*3/4, minRecordSummaryTokens)
	}
	if start > 0 && start < minRecordSummaryTokens {
		out = append(out, start)
	}
	return out
}

// estimateText is the repo's four-characters-per-token estimate for a string.
func estimateText(s string) int {
	return (len(s) + 3) / 4
}
