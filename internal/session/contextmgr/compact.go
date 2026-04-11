package contextmgr

import (
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// CompactResult holds the outcome of a compaction operation.
type CompactResult struct {
	Messages                  []types.Message
	SummaryMessages           []types.Message
	PreCompactTokenCount      int
	PostCompactTokenCount     int
	TruePostCompactTokenCount int
	Attachments               []types.Message
	HookResults               []types.Message
}

// Compactor defines the three-layer context compression strategy.
type Compactor struct {
	// Thresholds
	AutoCompactThreshold int
	BlockingLimit        int
}

// NewCompactor creates a compactor with default thresholds.
func NewCompactor() *Compactor {
	return &Compactor{
		AutoCompactThreshold: 120000,
		BlockingLimit:        180000,
	}
}

// AutoCompact triggers summarization when token count exceeds threshold.
// In a full implementation, this calls the LLM to summarize older messages.
func (c *Compactor) AutoCompact(messages []types.Message) (*CompactResult, error) {
	tokens := estimateTokens(messages)
	if tokens < c.AutoCompactThreshold {
		return nil, nil // No compaction needed
	}

	// Strategy: keep recent messages intact, summarize older ones
	boundary := findCompactBoundary(messages)
	if boundary <= 0 {
		return nil, nil
	}

	older := messages[:boundary]
	recent := messages[boundary:]

	// Placeholder: in production, send older messages to LLM for summary
	summary := types.Message{
		Role:    types.RoleSystem,
		Content: []types.ContentBlock{types.TextBlock{Text: "(conversation compacted: " + summarizeMessages(older) + ")"}},
	}

	result := &CompactResult{
		Messages:                  append([]types.Message{summary}, recent...),
		SummaryMessages:           []types.Message{summary},
		PreCompactTokenCount:      tokens,
		PostCompactTokenCount:     estimateTokens(recent) + 50,
		TruePostCompactTokenCount: estimateTokens(recent) + 50,
	}
	return result, nil
}

// SnipCompact removes zombie messages and stale markers.
// Feature-gated in Claude Code as HISTORY_SNIP.
func (c *Compactor) SnipCompact(messages []types.Message) ([]types.Message, int) {
	// Pattern: remove consecutive system messages that are just compact boundaries
	// and tool_result messages with zero content that precede them.
	out := make([]types.Message, 0, len(messages))
	tokensFreed := 0
	for _, m := range messages {
		if isZombieMessage(m) {
			tokensFreed += estimateMessageTokens(m)
			continue
		}
		out = append(out, m)
	}
	return out, tokensFreed
}

// ContextCollapse applies advanced context restructuring.
// Feature-gated in Claude Code as CONTEXT_COLLAPSE.
func (c *Compactor) ContextCollapse(messages []types.Message) []types.Message {
	// Pattern: project a collapsed view over the full history.
	// Summary messages live in a collapse store, not the main array.
	// This is a no-op in the base implementation (extension point).
	return messages
}

func findCompactBoundary(messages []types.Message) int {
	// Simple heuristic: compact everything except the last 4 messages
	if len(messages) <= 4 {
		return 0
	}
	return len(messages) - 4
}

func estimateTokens(messages []types.Message) int {
	// Very rough heuristic: 4 chars per token
	total := 0
	for _, m := range messages {
		total += estimateMessageTokens(m)
	}
	return total
}

func estimateMessageTokens(m types.Message) int {
	total := 0
	for _, b := range m.Content {
		if tb, ok := b.(types.TextBlock); ok {
			total += len(tb.Text) / 4
		}
	}
	return total
}

func isZombieMessage(m types.Message) bool {
	// A zombie is a compact boundary with no semantic content
	if m.Role != types.RoleSystem {
		return false
	}
	for _, b := range m.Content {
		if tb, ok := b.(types.TextBlock); ok {
			if tb.Text == "" || tb.Text == "(compact boundary)" {
				return true
			}
		}
	}
	return false
}

func summarizeMessages(msgs []types.Message) string {
	// In production, this is an LLM call.
	return "prior conversation summarized"
}
