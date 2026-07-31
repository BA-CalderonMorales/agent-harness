package tools

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestContentBudgetPropertyBased(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MaxSize = 100
	properties := gopter.NewProperties(parameters)

	// Invariant 1: NewContentBudget with non-positive limit falls back to DefaultMaxCharsPerTurn
	properties.Property("NewContentBudget: non-positive defaults to DefaultMaxCharsPerTurn", prop.ForAll(
		func(limit int) bool {
			budget := NewContentBudget(limit)
			_, max := budget.CurrentUsage()
			return max == DefaultMaxCharsPerTurn
		},
		gen.IntRange(-1000, 0),
	))

	// Invariant 2: Positive limits are respected directly
	properties.Property("NewContentBudget: positive limit is preserved", prop.ForAll(
		func(limit int) bool {
			budget := NewContentBudget(limit)
			_, max := budget.CurrentUsage()
			return max == limit
		},
		gen.IntRange(1, 1000000),
	))

	// Invariant 3: Exempt tools always pass CanUseResult
	properties.Property("CanUseResult: exempt tool always passes", prop.ForAll(
		func(toolName string, size int) bool {
			if toolName == "" {
				return true
			}
			budget := NewContentBudget(100)
			budget.MarkToolExempt(toolName)
			return budget.CanUseResult(toolName, size, 100)
		},
		gen.AlphaString(),
		gen.IntRange(1, 100000),
	))

	// Invariant 4: Non-exempt tool under limit is admitted
	properties.Property("RecordUsage: admits usage within budget", prop.ForAll(
		func(limit int, usage int) bool {
			budget := NewContentBudget(limit)
			toolMax := int64(limit)
			err := budget.RecordUsage("mytool", usage, toolMax)
			if usage <= limit {
				used, _ := budget.CurrentUsage()
				return err == nil && used == usage
			}
			return err != nil
		},
		gen.IntRange(100, 1000),
		gen.IntRange(1, 2000),
	))

	// Invariant 5: Reset resets usedChars to 0
	properties.Property("Reset: resets usage to 0", prop.ForAll(
		func(usage int) bool {
			budget := NewContentBudget(5000)
			_ = budget.RecordUsage("tool", usage, 5000)
			budget.Reset()
			used, _ := budget.CurrentUsage()
			return used == 0
		},
		gen.IntRange(1, 4000),
	))

	properties.TestingRun(t)
}
