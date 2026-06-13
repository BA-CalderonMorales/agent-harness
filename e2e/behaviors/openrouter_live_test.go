package behaviors

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func TestOpenRouterLiveStreamSmoke(t *testing.T) {
	if os.Getenv("AH_E2E_OPENROUTER") != "1" {
		t.Skip("set AH_E2E_OPENROUTER=1 to run OpenRouter-backed e2e")
	}

	apiKey := firstEnv("AH_API_KEY", "AGENT_HARNESS_API_KEY", "OPENROUTER_API_KEY")
	if apiKey == "" {
		t.Skip("OpenRouter e2e requires AH_API_KEY, AGENT_HARNESS_API_KEY, or OPENROUTER_API_KEY")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := llm.NewHTTPClient("openrouter", apiKey)
	events, err := client.Stream(ctx, llm.Request{
		Model:     firstNonEmpty(os.Getenv("AH_E2E_OPENROUTER_MODEL"), "openai/gpt-4.1-nano"),
		MaxTokens: 16,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.TextBlock{Text: "Reply with exactly: agent-harness-ok"}},
		}},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var text string
	stopped := false
	for event := range events {
		switch ev := event.(type) {
		case types.LLMTextDelta:
			text += ev.Delta
		case types.LLMMessageStop:
			stopped = true
		case types.LLMError:
			t.Fatalf("provider error = %v", ev.Error)
		}
	}

	if text == "" {
		t.Fatal("stream produced no text")
	}
	if !stopped {
		t.Fatal("stream ended without LLMMessageStop")
	}
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
