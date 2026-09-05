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

// TestTurnTextSegmentsGetBeats pins the UX fix: within one turn, the
// response segments where tool calls interrupted it — Parts carry the
// structure, Content stays one clean document, and no synthetic beat
// text rides the stream.
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

	// The finalized assistant message: Content stays one clean
	// document (no beat text), and Parts carry the segmentation.
	content := ""
	var parts []tui.TurnPart
	for _, m := range chatMessages(t, tuiApp) {
		if m.role == "assistant" {
			content = m.content
			parts = m.parts
		}
	}
	if strings.Contains(content, "· · ·") {
		t.Fatalf("synthetic beat leaked into the assistant content:\n%q", content)
	}
	for _, want := range []string{
		"Let me take a look around.",
		"Let me peek further.",
		"Here is the tour.",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("segment text missing %q:\n%q", want, content)
		}
	}

	// The parts interleave: tool t1, text, tool t2, text — the last
	// two prose runs merge (no call between them).
	if len(parts) != 4 {
		t.Fatalf("parts = %d, want 4 (tool/text/tool/text)", len(parts))
	}
	if parts[0].ToolID != "t1" || parts[2].ToolID != "t2" {
		t.Fatalf("tool parts = [%q, %q], want [t1, t2]", parts[0].ToolID, parts[2].ToolID)
	}
	if !strings.Contains(parts[1].Text, "Let me take a look around.") {
		t.Fatalf("first prose part = %q", parts[1].Text)
	}
	if !strings.Contains(parts[3].Text, "Let me peek further.") || !strings.Contains(parts[3].Text, "Here is the tour.") {
		t.Fatalf("final prose part = %q", parts[3].Text)
	}
}
