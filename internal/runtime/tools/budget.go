package tools

import (
	"fmt"
	"sync"
)

// ContentBudget tracks aggregate tool result sizes per turn to prevent context explosion.
// Tools with MaxResultSizeChars == 0 or marked as infinite are exempt.
type ContentBudget struct {
	mu              sync.RWMutex
	usedChars       int
	maxCharsPerTurn int
	exemptTools     map[string]bool // Tools exempt from budget
}

// DefaultMaxCharsPerTurn is the default budget limit per turn.
const DefaultMaxCharsPerTurn = 50000

// NewContentBudget creates a new budget tracker.
func NewContentBudget(maxChars int) *ContentBudget {
	if maxChars <= 0 {
		maxChars = DefaultMaxCharsPerTurn
	}
	return &ContentBudget{
		maxCharsPerTurn: maxChars,
		exemptTools:     make(map[string]bool),
	}
}

// MarkToolExempt marks a tool as exempt from budget tracking.
// Exempt tools are typically read tools (file_read, glob, grep) where
// the content is necessary for the agent to function.
func (b *ContentBudget) MarkToolExempt(toolName string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.exemptTools[toolName] = true
}

// CanUseResult checks if a tool result fits within the budget.
// Returns true if the result can be added, false if it would exceed budget.
func (b *ContentBudget) CanUseResult(toolName string, resultSize int, toolMaxResultSize int64) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Tool is explicitly exempt
	if b.exemptTools[toolName] {
		return true
	}

	// Tool has infinite result size (MaxResultSizeChars == 0 or very large)
	if toolMaxResultSize == 0 || toolMaxResultSize > 1000000 {
		return true
	}

	// Check if this result would exceed budget
	if b.usedChars+resultSize > b.maxCharsPerTurn {
		return false
	}

	return true
}

// RecordUsage records that a tool result of the given size was used.
// Returns an error if the result exceeds budget (unless exempt).
func (b *ContentBudget) RecordUsage(toolName string, resultSize int, toolMaxResultSize int64) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Exempt tools don't count against budget
	if b.exemptTools[toolName] {
		return nil
	}
	if toolMaxResultSize == 0 || toolMaxResultSize > 1000000 {
		return nil
	}

	if b.usedChars+resultSize > b.maxCharsPerTurn {
		return fmt.Errorf("content budget exceeded: used %d/%d chars, result is %d chars",
			b.usedChars, b.maxCharsPerTurn, resultSize)
	}

	b.usedChars += resultSize
	return nil
}

// CurrentUsage returns the current budget usage.
func (b *ContentBudget) CurrentUsage() (used, max int) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.usedChars, b.maxCharsPerTurn
}

// Reset clears the budget for a new turn.
func (b *ContentBudget) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.usedChars = 0
}

// GetTruncatedResult truncates a result to fit the remaining budget.
// Returns the truncated result and a note about truncation.
func (b *ContentBudget) GetTruncatedResult(toolName string, result string, toolMaxResultSize int64) (string, string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Don't truncate exempt tools
	if b.exemptTools[toolName] || toolMaxResultSize == 0 || toolMaxResultSize > 1000000 {
		return result, ""
	}

	remaining := b.maxCharsPerTurn - b.usedChars
	if len(result) <= remaining {
		return result, ""
	}

	// Truncate with note
	truncated := result[:remaining]
	note := fmt.Sprintf("\n\n[Content truncated: result exceeded turn budget. Used %d/%d chars]",
		b.usedChars+len(result), b.maxCharsPerTurn)

	b.usedChars += remaining

	return truncated + note, note
}

// Global budget instance for the current turn
var currentBudget = NewContentBudget(DefaultMaxCharsPerTurn)

// InitDefaultBudget sets up the default budget with standard exemptions.
func InitDefaultBudget() {
	// Read/search tools are exempt - their content is essential
	currentBudget.MarkToolExempt("read")
	currentBudget.MarkToolExempt("glob")
	currentBudget.MarkToolExempt("grep")
	currentBudget.MarkToolExempt("search")
	currentBudget.MarkToolExempt("web_fetch")
	currentBudget.MarkToolExempt("web_search")
}

// GetCurrentBudget returns the global budget instance.
func GetCurrentBudget() *ContentBudget {
	return currentBudget
}

// ResetBudgetForNewTurn resets the budget for a new agent turn.
func ResetBudgetForNewTurn() {
	currentBudget.Reset()
}

func init() {
	InitDefaultBudget()
}
