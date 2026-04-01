package contextmgr

import (
	"unicode/utf8"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// TokenEstimator provides token count heuristics.
type TokenEstimator struct {
	// CharsPerToken is the rough character-to-token ratio.
	// For English text, ~4 chars per token is common.
	CharsPerToken float64
}

// NewTokenEstimator creates an estimator with default ratios.
func NewTokenEstimator() *TokenEstimator {
	return &TokenEstimator{CharsPerToken: 4.0}
}

// EstimateMessages counts tokens for a slice of messages.
func (e *TokenEstimator) EstimateMessages(msgs []types.Message) int {
	total := 0
	for _, m := range msgs {
		total += e.estimateMessage(m)
	}
	return total
}

// estimateMessage counts tokens for a single message.
func (e *TokenEstimator) estimateMessage(m types.Message) int {
	// Base overhead per message (~3 tokens)
	total := 3
	for _, block := range m.Content {
		switch b := block.(type) {
		case types.TextBlock:
			total += e.estimateText(b.Text)
		case types.ToolUseBlock:
			total += e.estimateText(b.Name)
			total += e.estimateMap(b.Input)
		case types.ToolResultBlock:
			total += e.estimateText(b.Content)
		case types.ThinkingBlock:
			total += e.estimateText(b.Thinking)
		}
	}
	return total
}

// estimateText counts tokens for a string.
func (e *TokenEstimator) estimateText(text string) int {
	if text == "" {
		return 0
	}
	chars := utf8.RuneCountInString(text)
	return int(float64(chars) / e.CharsPerToken)
}

// estimateMap counts tokens for a map (simplified).
func (e *TokenEstimator) estimateMap(m map[string]any) int {
	total := 0
	for k, v := range m {
		total += e.estimateText(k)
		if s, ok := v.(string); ok {
			total += e.estimateText(s)
		}
	}
	return total
}

// IsNearLimit returns true if tokens are within a threshold of the limit.
func (e *TokenEstimator) IsNearLimit(tokens, limit int) bool {
	return tokens >= limit
}
