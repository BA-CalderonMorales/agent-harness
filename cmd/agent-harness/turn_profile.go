package main

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/agent/conversation"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
)

// Greetings still use the selected model and conversation history. Only the
// coding instructions, skill bodies and tool schemas are omitted for this turn.
// Questions and compound greetings keep the normal coding path.
func (app *App) turnProfile(input string) (string, []tools.Tool, int) {
	if conversation.ClassifyInput(input) == conversation.TypeGreeting {
		maxOutput := app.config.MaxTokens
		if maxOutput <= 0 || maxOutput > 256 {
			maxOutput = 256
		}
		return agent.GetMinimalSystemPrompt("Agent"), nil, maxOutput
	}
	return app.buildSystemPrompt(), app.enabledToolsForMode(), app.config.MaxTokens
}

// fitOutputBudget keeps the reply inside the window the model actually
// has after the fixed prompt and the conversation. Configuring a 4k
// local model with a 4k max_tokens was a request for 8k of tokens in a
// 4k window; the provider truncates, errors, or degrades. The floor
// keeps a usable reply when history is momentarily tight - compaction
// owns the history side.
func fitOutputBudget(contextLength, promptTokens, historyTokens, requested int) int {
	if contextLength <= 0 || requested <= 0 {
		return requested
	}
	available := contextLength - promptTokens - historyTokens
	if available < 256 {
		return 256
	}
	if requested > available {
		return available
	}
	return requested
}
