package agent

import (
	"math"
	"testing"
)

// TestFreeModelsCostNothing pins the footer lie from the OpenRouter
// paste: a :free model was priced as Claude Sonnet because it matched no
// pricing branch, so the first turn charged a fabricated $0.04.
func TestFreeModelsCostNothing(t *testing.T) {
	usage := TokenUsage{InputTokens: 4000, OutputTokens: 1000}
	for _, model := range []string{
		"nvidia/nemotron-3.5-lightning:free",
		"nvidia/nemotron-3-super-120b-a12b:free",
		"",
	} {
		if cost := usage.CostUSD(model); cost != 0 {
			t.Errorf("CostUSD(%q) = %v, want 0", model, cost)
		}
	}
}

// TestPaidModelsKeepAnEstimate guards the other direction: the free-tier
// zero must not blank out a paid model's footer cost.
func TestPaidModelsKeepAnEstimate(t *testing.T) {
	usage := TokenUsage{InputTokens: 1_000_000, OutputTokens: 1_000_000}
	if cost := usage.CostUSD("claude-3-5-sonnet-20241022"); math.Abs(cost-18.0) > 0.001 {
		t.Fatalf("CostUSD(sonnet) = %v, want 18", cost)
	}
}
