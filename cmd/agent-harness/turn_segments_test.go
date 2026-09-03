package main

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// segmentedClient replays the pattern from session 4d06abe0: a text
// segment, a tool call, another text segment, a second tool call, and
// the final summary — three text segments separated by tools in one
// turn.
type segmentedClient struct {
	llm.Client
	calls int
}

func (c *segmentedClient) Stream(ctx context.Context, req llm.Request) (<-chan types.LLMEvent, error) {
	c.calls++
	out := make(chan types.LLMEvent, 64)
	go func() {
		defer close(out)
		switch c.calls {
		case 1:
			out <- types.LLMMessageStart{ID: "m1"}
			out <- types.LLMToolUseDelta{ID: "t1", Name: "echo", Delta: `{"text":"one"}`}
			out <- types.LLMTextDelta{Delta: "Let me take a look around."}
			out <- types.LLMMessageStop{StopReason: "tool_calls"}
		case 2:
			out <- types.LLMMessageStart{ID: "m2"}
			out <- types.LLMToolUseDelta{ID: "t2", Name: "echo", Delta: `{"text":"two"}`}
			out <- types.LLMTextDelta{Delta: "Let me peek further."}
			out <- types.LLMMessageStop{StopReason: "tool_calls"}
		default:
			out <- types.LLMMessageStart{ID: "m3"}
			out <- types.LLMTextDelta{Delta: "Here is the tour."}
			out <- types.LLMMessageStop{StopReason: "stop"}
		}
	}()
	return out, nil
}

// TestTurnTextSegmentsGetBeats pins the UX fix: within one turn, a text
// segment that follows tool activity is separated from the previous one
// by a visible beat — never glued into one meaningless sentence wall.
func TestTurnTextSegmentsGetBeats(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "local"}, "test-model")
	tuiApp := tui.NewApp()
	app.tuiApp = tuiApp
	app.loop = agent.NewLoop(&segmentedClient{})
	app.toolRegistry = tools.NewRegistry()
	app.toolRegistry.RegisterBuiltIn(echoTool())

	app.handleAgentLoopAsync("tour the codebase", tuiApp)

	// Drain the turn through the real TUI update path, accumulating the
	// stream buffer exactly like the live event loop.
	var model tea.Model = tuiApp
	deadline := time.Now().Add(5 * time.Second)
	done := false
	for time.Now().Before(deadline) && !done {
		msg := receiveTUIMessage(t, tuiApp)
		model, _ = model.Update(msg)
		if _, ok := msg.(tui.AgentDoneMsg); ok {
			done = true
		}
	}
	if !done {
		t.Fatal("turn did not settle")
	}

	// The finalized assistant content: the segments must be separated
	// by the beat, and the session-facing response must stay clean.
	content := ""
	for _, m := range chatMessages(t, tuiApp) {
		if m.role == "assistant" {
			content = m.content
		}
	}
	if !strings.Contains(content, "Let me take a look around.\n\n· · ·\n\nLet me peek further.") {
		t.Fatalf("segment beats missing from the assistant bubble:\n%q", content)
	}
	if !strings.Contains(content, "· · ·\n\nHere is the tour.") {
		t.Fatalf("final segment not separated:\n%q", content)
	}
	if strings.Count(content, "· · ·") != 2 {
		t.Fatalf("expected exactly 2 beats, got %d:\n%q", strings.Count(content, "· · ·"), content)
	}
}
