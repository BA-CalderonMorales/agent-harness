package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	tea "github.com/charmbracelet/bubbletea"
)

// burstClient serves one assistant message per request: the first yields
// burstSize tool calls (over the loop's default ceiling), the follow-up
// yields a text reply.
type burstClient struct {
	llm.Client
	first     bool
	burstSize int
}

func (c *burstClient) Stream(ctx context.Context, req llm.Request) (<-chan types.LLMEvent, error) {
	first := c.first
	c.first = false // synchronous: no goroutine race on the flag
	out := make(chan types.LLMEvent, 64)
	go func() {
		defer close(out)
		if first {
			out <- types.LLMMessageStart{ID: "m1"}
			for i := 0; i < c.burstSize; i++ {
				out <- types.LLMToolUseDelta{ID: fmt.Sprintf("t%d", i), Name: "echo", Delta: fmt.Sprintf(`{"text":"%d"}`, i)}
			}
			out <- types.LLMMessageStop{StopReason: "tool_calls"}
			return
		}
		out <- types.LLMMessageStart{ID: "m2"}
		out <- types.LLMTextDelta{Delta: "done"}
		out <- types.LLMMessageStop{StopReason: "stop"}
	}()
	return out, nil
}

// TestLimitNoticeRendersAsSystemMessage pins the role-routing fix end to
// end: when the loop trips the tool ceiling, its notice is a system-role
// StreamMessage and must land in the chat as a system message - never
// stream as a fake assistant reply with a chunk counter, and never leave
// the user staring at a silent cut-off turn.
func TestLimitNoticeRendersAsSystemMessage(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "local"}, "test-model")
	tuiApp := tui.NewApp()
	app.tuiApp = tuiApp
	app.client = &burstClient{first: true, burstSize: 20} // trips the default ceiling of 15
	app.loop = agent.NewLoop(app.client)
	app.toolRegistry = tools.NewRegistry()
	app.toolRegistry.RegisterBuiltIn(echoTool())

	app.handleAgentLoopAsync("explore", tuiApp)

	// Drain until the turn settles, feeding every message through the
	// TUI so the chat model actually processes it.
	var model tea.Model = tuiApp
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		msg := receiveTUIMessage(t, tuiApp)
		model, _ = model.Update(msg)
		if _, ok := msg.(tui.AgentDoneMsg); ok {
			break
		}
	}

	sawSystem := false
	for _, m := range chatMessages(t, tuiApp) {
		if m.role == "assistant" && strings.Contains(m.content, "Tool call limit reached") {
			t.Fatalf("limit notice rendered as an assistant reply: %q", m.content)
		}
		if m.role == "system" && strings.Contains(m.content, "Tool call limit reached") {
			sawSystem = true
		}
	}
	if !sawSystem {
		t.Fatal("limit notice never rendered as a system message")
	}
}

func echoTool() tools.Tool {
	return tools.Tool{
		Name:        "echo",
		Description: "echo",
		InputSchema: func() map[string]any {
			return map[string]any{"type": "object", "properties": map[string]any{"text": map[string]any{"type": "string"}}}
		},
		ValidateInput: func(input map[string]any, ctx tools.Context) tools.ValidationResult {
			return tools.ValidationResult{Valid: true}
		},
		UserFacingName:         func(input map[string]any) string { return "Echo" },
		GetActivityDescription: func(input map[string]any) string { return "echo" },
		Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
			return tools.ToolResult{Data: "ok"}, nil
		},
		MapResult: func(data any, toolUseID string) types.ToolResultBlock {
			return types.ToolResultBlock{ToolUseID: toolUseID, Content: fmt.Sprintf("%v", data)}
		},
		Capabilities: tools.CapabilityFlags{IsReadOnly: func(map[string]any) bool { return true }},
	}
}
