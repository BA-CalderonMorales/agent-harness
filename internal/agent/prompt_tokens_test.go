package agent

import (
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// TestEstimatePromptTokensCountsSchemas pins the fixed-cost estimate: the
// system prompt alone is not the request. Tool names, descriptions, and
// JSON schemas ride on every call and must count against the window.
func TestEstimatePromptTokensCountsSchemas(t *testing.T) {
	tool := tools.NewTool(tools.Tool{
		Name:        "read",
		Description: "Read a file from disk.",
		InputSchema: func() map[string]any {
			return map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file_path": map[string]any{"type": "string"},
				},
			}
		},
	})

	withoutTools := EstimatePromptTokens("system", nil)
	withTools := EstimatePromptTokens("system", []tools.Tool{tool})
	if withTools <= withoutTools {
		t.Fatalf("tool schema was not counted: without=%d with=%d", withoutTools, withTools)
	}
}

// TestBlockingLimitIncludesPromptOverhead is the over-budget repro: a
// 33k-character prompt plus tool schemas was invisible to the loop, so a
// request the provider counted at ~10k tokens looked comfortably under
// an 8k limit. The fixed prompt must be part of the comparison.
func TestBlockingLimitIncludesPromptOverhead(t *testing.T) {
	loop := NewLoop(nil)
	loop.Config.BlockingTokenLimit = 100

	msgs := []types.Message{{
		Role:    types.RoleUser,
		Content: []types.ContentBlock{types.TextBlock{Text: strings.Repeat("x", 300)}},
	}} // ~75 tokens

	if !loop.isAtBlockingLimit(msgs, 30) {
		t.Fatal("75 message tokens + 30 prompt tokens must exceed a 100-token limit")
	}
	if loop.isAtBlockingLimit(msgs, 20) {
		t.Fatal("75 message tokens + 20 prompt tokens must stay under a 100-token limit")
	}
}
