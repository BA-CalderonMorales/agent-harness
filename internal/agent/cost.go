package agent

import (
	"fmt"
	"sync"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// CostTracker accumulates API usage costs across a session.
type CostTracker struct {
	mu            sync.Mutex
	InputTokens   int
	OutputTokens  int
	CacheReadTokens int
	CacheCreationTokens int
	EstimatedUSD  float64
}

// NewCostTracker creates a fresh cost tracker.
func NewCostTracker() *CostTracker {
	return &CostTracker{}
}

// RecordUsage adds token usage to the tracker.
func (c *CostTracker) RecordUsage(usage types.TokenUsage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.InputTokens += usage.InputTokens
	c.OutputTokens += usage.OutputTokens
	c.CacheReadTokens += usage.CacheReadInputTokens
	c.CacheCreationTokens += usage.CacheCreationInputTokens
	c.EstimatedUSD += c.estimateCost(usage)
}

// Summary returns a human-readable cost summary.
func (c *CostTracker) Summary() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return fmt.Sprintf(
		"Tokens: %d input, %d output, %d cache read, %d cache creation | Est. $%.4f",
		c.InputTokens, c.OutputTokens, c.CacheReadTokens, c.CacheCreationTokens, c.EstimatedUSD,
	)
}

// estimateCost uses rough pricing heuristics.
func (c *CostTracker) estimateCost(usage types.TokenUsage) float64 {
	// Rough Claude 3.5 Sonnet pricing (per 1M tokens)
	inputRate := 3.0 / 1_000_000.0
	outputRate := 15.0 / 1_000_000.0
	cacheReadRate := 0.30 / 1_000_000.0
	cacheCreationRate := 3.75 / 1_000_000.0

	cost := float64(usage.InputTokens) * inputRate
	cost += float64(usage.OutputTokens) * outputRate
	cost += float64(usage.CacheReadInputTokens) * cacheReadRate
	cost += float64(usage.CacheCreationInputTokens) * cacheCreationRate
	return cost
}
