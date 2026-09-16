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

// TestSystemPromptAgreesOnTheGoalBeforeActing pins the steering the user
// asked for: answer first, understand before changing, and write the plan
// with todo_write before the first mutating call — so the agent guides the
// user toward the goal instead of guessing at it with tools.
func TestSystemPromptAgreesOnTheGoalBeforeActing(t *testing.T) {
	prompt := strings.Join(strings.Fields(BuildSystemPrompt(SystemPromptConfig{})), " ")
	for _, want := range []string{
		"## Before You Act",
		"Answer first, then work",
		"Understand before changing",
		"ask ONE specific question and stop",
		"never ask a question after you have already begun changing files",
		"call todo_write BEFORE the first mutating tool call",
		"exactly one item in_progress",
		"Keep the list current",
		"never delete an item to hide it",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("system prompt missing the before-you-act contract %q", want)
		}
	}
}

func TestSystemPromptContinuesAcceptedWorkWithDurableHandoff(t *testing.T) {
	prompt := strings.Join(strings.Fields(BuildSystemPrompt(SystemPromptConfig{})), " ")
	for _, want := range []string{
		"For accepted multi-step work, pursue the objective until it is complete",
		"batch independent reads",
		"adapt after failed edits or",
		"distinguish partial progress from verified completion",
		"checkpoint it after meaningful milestones and before expensive work",
		"On continuation or resume, read that ledger and verify the current state",
		"and do not promise that a prompt or context exhaustion will trigger a final write",
		"Runtime limits, cancellation, permissions, and convergence guards",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("system prompt missing continuation contract %q", want)
		}
	}
	if strings.Contains(prompt, "After 3-4 tool attempts") || strings.Contains(prompt, "After 3–4 tool attempts") {
		t.Fatal("system prompt retains the arbitrary 3–4-tool stopping rule")
	}
}
