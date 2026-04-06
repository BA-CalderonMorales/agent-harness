package permissions

import (
	"context"
	"fmt"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// ClassifierResult is the outcome of the auto-mode safety classifier.
type ClassifierResult struct {
	Safe   bool
	Reason string
}

// AutoClassifier evaluates whether a tool action is safe in auto mode.
// In production, this makes a separate LLM call. Here we demonstrate the pattern.
type AutoClassifier struct {
	// Client would be an LLM client in a full implementation
}

// NewAutoClassifier creates a classifier instance.
func NewAutoClassifier() *AutoClassifier {
	return &AutoClassifier{}
}

// ClassifyAction determines if a tool use should proceed without user confirmation.
func (c *AutoClassifier) ClassifyAction(ctx context.Context, messages []types.Message, toolName string, input map[string]any, availableTools []tools.Tool) (ClassifierResult, error) {
	// Fast path: certain tools are always safe
	if isAlwaysSafeTool(toolName) {
		return ClassifierResult{Safe: true, Reason: "always-safe tool"}, nil
	}

	// Fast path: read-only operations on safe tools
	if isReadOnlyOperation(toolName, input) {
		return ClassifierResult{Safe: true, Reason: "read-only operation"}, nil
	}

	// Pattern: build a mini-prompt from the last few messages and the action
	var transcript strings.Builder
	for _, m := range messages {
		if len(m.Content) > 0 {
			if tb, ok := m.Content[0].(types.TextBlock); ok {
				transcript.WriteString(fmt.Sprintf("%s: %s\n", m.Role, tb.Text))
			}
		}
	}
	transcript.WriteString(fmt.Sprintf("\nProposed action: %s(%+v)\n", toolName, input))
	transcript.WriteString("Is this action safe to execute without asking the user?\n")

	// In a full implementation, send transcript to a small/fast LLM
	_ = transcript.String()

	// Conservative default: block unless we're confident
	return ClassifierResult{
		Safe:   false,
		Reason: "classifier not implemented; defaulting to ask",
	}, nil
}

func isAlwaysSafeTool(toolName string) bool {
	safe := map[string]bool{
		"todo_write":      true,
		"enter_plan_mode": true,
		"exit_plan_mode":  true,
	}
	return safe[toolName]
}

func isReadOnlyOperation(toolName string, input map[string]any) bool {
	readOnlyTools := map[string]bool{
		"read":       true,
		"glob":       true,
		"grep":       true,
		"web_fetch":  true,
		"web_search": true,
	}
	return readOnlyTools[toolName]
}
