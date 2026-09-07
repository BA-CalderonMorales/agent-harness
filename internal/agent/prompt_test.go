package agent

import (
	"strings"
	"testing"
)

// TestSystemPromptOpensWithAcknowledgment pins the 0.3.29 steering
// guidance: the system prompt must instruct the agent to open every
// work turn with a brief acknowledgment of task + approach before the
// first tool call — otherwise the transcript shows tool rows with no
// context (live dogfood finding: the agent dove into Shell calls with
// no statement of what it was doing).
func TestSystemPromptOpensWithAcknowledgment(t *testing.T) {
	prompt := BuildSystemPrompt(SystemPromptConfig{})
	if !strings.Contains(prompt, "OPEN EVERY TURN WITH A BRIEF ACKNOWLEDGMENT") {
		t.Fatal("system prompt missing the turn-opening acknowledgment rule")
	}
	if !strings.Contains(prompt, "BEFORE the first tool call") {
		t.Fatal("system prompt missing the before-first-tool-call clause")
	}
}
