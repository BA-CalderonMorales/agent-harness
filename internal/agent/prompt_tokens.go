package agent

import (
	"encoding/json"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
)

// EstimatePromptTokens estimates the fixed part of a request: the system
// prompt plus the JSON schemas and descriptions of every tool offered.
// Context accounting used to look only at conversation messages, so a
// turn with a 30k-character system prompt and thirteen tool schemas
// reported 2k tokens used while the provider counted ~10k. The budget
// has to include what the provider actually receives.
func EstimatePromptTokens(systemPrompt string, toolSet []tools.Tool) int {
	characters := len(systemPrompt)
	for _, tool := range toolSet {
		characters += len(tool.Name) + len(tool.Description)
		schema := tool.InputSchema()
		if schema == nil && tool.InputJSONSchema != nil {
			schema = tool.InputJSONSchema
		}
		if schema == nil {
			continue
		}
		if raw, err := json.Marshal(schema); err == nil {
			characters += len(raw)
		}
	}
	// Four characters per token matches messages.EstimateTokens, the
	// project's single approximation. Round up so a short prompt is
	// never counted as zero.
	return (characters + 3) / 4
}
