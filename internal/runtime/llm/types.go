package llm

import (
	"context"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// Client abstracts the language model provider.
type Client interface {
	// Stream sends a request and yields response events.
	Stream(ctx context.Context, req Request) (<-chan types.LLMEvent, error)
}

// Request is the payload sent to the LLM.
type Request struct {
	Messages       []types.Message
	SystemPrompt   string
	Tools          []tools.Tool
	Model          string
	MaxTokens      int
	Temperature    float64
	ThinkingBudget int
}
