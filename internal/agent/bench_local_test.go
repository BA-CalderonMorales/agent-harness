package agent

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// TestBenchLocalGemma drives the real harness loop against a local
// OpenAI-compatible llama.cpp server and reports wall-clock outcome.
func TestBenchLocalGemma(t *testing.T) {
	baseURL := os.Getenv("AH_ENDPOINT_URL")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8080/v1"
	}
	client := llm.NewHTTPClientWithBaseURL("local", "", baseURL)
	client.HTTPClient.Timeout = 20 * time.Minute
	loop := NewLoop(client)

	prompt := "Explore this repository: list the top-level files, read the Makefile, and report what the default target does. Give the answer directly without reasoning preamble."
	start := time.Now()
	out, err := loop.Query(context.Background(), QueryParams{
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.TextBlock{Text: prompt}},
		}},
		CanUseTool: tools.CanUseToolFn(func(name string, input map[string]any, ctx tools.Context) (tools.PermissionDecision, error) {
			return tools.PermissionDecision{Behavior: tools.Allow}, nil
		}),
		ToolUseContext:  tools.Context{},
		MaxOutputTokens: 1600,
		MaxTurns:        10,
	})
	if err != nil {
		t.Fatalf("query start failed: %v", err)
	}

	events := 0
	terminalReason := ""
	var terminal *types.Message
	for ev := range out {
		events++
		if term, ok := ev.(types.StreamTerminal); ok {
			terminalReason = string(term.Reason)
			terminal = term.Message
		}
	}

	elapsed := time.Since(start)
	msg := ""
	if terminal != nil {
		msg = fmt.Sprint(terminal.Content)
	}
	t.Logf("events=%d terminal=%q elapsed=%s", events, terminalReason, elapsed)
	if msg != "" {
		t.Logf("terminal message: %.200s", msg)
	}
	fmt.Printf("BENCH events=%d terminal=%q elapsed=%s\n", events, terminalReason, elapsed)
}
