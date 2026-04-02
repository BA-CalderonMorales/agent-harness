// Cost tracking system inspired by claw-code

package agent

import (
	"fmt"
	"strings"
	"sync"
)

// TokenUsage tracks token usage for a single turn or cumulatively
type TokenUsage struct {
	InputTokens              int
	OutputTokens             int
	CacheCreationInputTokens int
	CacheReadInputTokens     int
}

// TotalTokens returns the total token count
func (u TokenUsage) TotalTokens() int {
	return u.InputTokens + u.OutputTokens + u.CacheReadInputTokens
}

// CostUSD estimates the cost in USD (rough estimates)
func (u TokenUsage) CostUSD(model string) float64 {
	// Very rough pricing estimates per 1M tokens
	var inputPrice, outputPrice float64
	
	switch {
	case stringContains(model, "claude-3-opus"):
		inputPrice = 15.0
		outputPrice = 75.0
	case stringContains(model, "claude-3-sonnet"):
		inputPrice = 3.0
		outputPrice = 15.0
	case stringContains(model, "claude-3-haiku"):
		inputPrice = 0.25
		outputPrice = 1.25
	case stringContains(model, "gpt-4o"):
		inputPrice = 5.0
		outputPrice = 15.0
	case stringContains(model, "gpt-4"):
		inputPrice = 30.0
		outputPrice = 60.0
	default:
		// Default to claude-sonnet pricing
		inputPrice = 3.0
		outputPrice = 15.0
	}

	inputCost := float64(u.InputTokens) / 1_000_000 * inputPrice
	outputCost := float64(u.OutputTokens) / 1_000_000 * outputPrice
	
	return inputCost + outputCost
}

// CostTracker tracks cumulative costs across a session
type CostTracker struct {
	mu         sync.RWMutex
	turns      []TokenUsage
	currentTurn TokenUsage
	model      string
}

// NewCostTracker creates a new cost tracker
func NewCostTracker() *CostTracker {
	return &CostTracker{
		turns: make([]TokenUsage, 0),
	}
}

// SetModel sets the model for cost estimation
func (ct *CostTracker) SetModel(model string) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.model = model
}

// RecordTurn records a completed turn
func (ct *CostTracker) RecordTurn(usage TokenUsage) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.turns = append(ct.turns, usage)
}

// AddToCurrentTurn adds usage to the current turn
func (ct *CostTracker) AddToCurrentTurn(usage TokenUsage) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.currentTurn.InputTokens += usage.InputTokens
	ct.currentTurn.OutputTokens += usage.OutputTokens
	ct.currentTurn.CacheCreationInputTokens += usage.CacheCreationInputTokens
	ct.currentTurn.CacheReadInputTokens += usage.CacheReadInputTokens
}

// CompleteTurn completes the current turn and starts a new one
func (ct *CostTracker) CompleteTurn() {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.turns = append(ct.turns, ct.currentTurn)
	ct.currentTurn = TokenUsage{}
}

// GetCurrentTurn returns the current turn usage
func (ct *CostTracker) GetCurrentTurn() TokenUsage {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	return ct.currentTurn
}

// GetCumulative returns cumulative usage across all turns
func (ct *CostTracker) GetCumulative() TokenUsage {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	
	var total TokenUsage
	for _, turn := range ct.turns {
		total.InputTokens += turn.InputTokens
		total.OutputTokens += turn.OutputTokens
		total.CacheCreationInputTokens += turn.CacheCreationInputTokens
		total.CacheReadInputTokens += turn.CacheReadInputTokens
	}
	// Include current turn
	total.InputTokens += ct.currentTurn.InputTokens
	total.OutputTokens += ct.currentTurn.OutputTokens
	total.CacheCreationInputTokens += ct.currentTurn.CacheCreationInputTokens
	total.CacheReadInputTokens += ct.currentTurn.CacheReadInputTokens
	
	return total
}

// GetTotalCost returns the total estimated cost
func (ct *CostTracker) GetTotalCost() float64 {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	
	var totalCost float64
	for _, turn := range ct.turns {
		totalCost += turn.CostUSD(ct.model)
	}
	totalCost += ct.currentTurn.CostUSD(ct.model)
	
	return totalCost
}

// GetTurns returns the number of completed turns
func (ct *CostTracker) GetTurns() int {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	return len(ct.turns)
}

// Summary returns a formatted summary string
func (ct *CostTracker) Summary() string {
	cumulative := ct.GetCumulative()
	totalCost := ct.GetTotalCost()
	turns := ct.GetTurns()
	
	return fmt.Sprintf("Cost: %d input + %d output tokens (~$%.4f) across %d turns",
		cumulative.InputTokens,
		cumulative.OutputTokens,
		totalCost,
		turns)
}

// FormatReport returns a detailed cost report
func (ct *CostTracker) FormatReport() string {
	cumulative := ct.GetCumulative()
	totalCost := ct.GetTotalCost()
	
	var result string
	result += "Cost\n"
	result += fmt.Sprintf("  Input tokens     %d\n", cumulative.InputTokens)
	result += fmt.Sprintf("  Output tokens    %d\n", cumulative.OutputTokens)
	if cumulative.CacheCreationInputTokens > 0 {
		result += fmt.Sprintf("  Cache create     %d\n", cumulative.CacheCreationInputTokens)
	}
	if cumulative.CacheReadInputTokens > 0 {
		result += fmt.Sprintf("  Cache read       %d\n", cumulative.CacheReadInputTokens)
	}
	result += fmt.Sprintf("  Total tokens     %d\n", cumulative.TotalTokens())
	if totalCost > 0 {
		result += fmt.Sprintf("  Est. cost        $%.4f\n", totalCost)
	}
	result += "\n"
	result += "Next\n"
	result += "  /status          See session + workspace context\n"
	result += "  /compact         Trim local history if the session is getting large\n"
	
	return result
}

// Reset resets the cost tracker
func (ct *CostTracker) Reset() {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.turns = make([]TokenUsage, 0)
	ct.currentTurn = TokenUsage{}
}

func stringContains(s, substr string) bool {
	return strings.Contains(s, substr)
}
