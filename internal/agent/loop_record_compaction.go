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
	budget := target/2 - estimateText(recordSummaryLead)
	if budget > maxRecordSummaryTokens {
		budget = maxRecordSummaryTokens
	}
	last := len(state.messages) - 1
	var after int
	for start := keepStart; start <= last; start++ {
		suffix := state.messages[start:]
		for tokens := budget; tokens >= minRecordSummaryTokens; tokens = tokens * 3 / 4 {
			body := record.Render(state.record, tokens)
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

// estimateText is the repo's four-characters-per-token estimate for a string.
func estimateText(s string) int {
	return (len(s) + 3) / 4
}
