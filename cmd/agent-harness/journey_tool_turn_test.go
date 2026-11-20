package main

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

const (
	journeyToolCommand = "echo journey-needle"
	journeyToolAnswer  = "The shell printed the needle."
)

// toolTurnClient answers the first request with one shell call and every
// later request with the final answer, the shape of a real tool turn.
type toolTurnClient struct {
	llm.Client
	mu    sync.Mutex
	calls int
	// sawResult records whether the follow-up request carried the
	// tool's output back to the model.
	sawResult bool
}

func (c *toolTurnClient) Stream(_ context.Context, req llm.Request) (<-chan types.LLMEvent, error) {
	c.mu.Lock()
	c.calls++
	call := c.calls
	if call > 1 {
		for _, m := range req.Messages {
			for _, block := range m.Content {
				if r, ok := block.(types.ToolResultBlock); ok && strings.Contains(r.Content, "journey-needle") {
					c.sawResult = true
				}
			}
		}
	}
	c.mu.Unlock()

	out := make(chan types.LLMEvent, 8)
	go func() {
		defer close(out)
		if call == 1 {
			out <- types.LLMMessageStart{ID: "journey-m1"}
			out <- types.LLMToolUseDelta{ID: "journey-t1", Name: "bash", Delta: `{"command":"` + journeyToolCommand + `"}`}
			out <- types.LLMMessageStop{StopReason: "tool_calls"}
			return
		}
		out <- types.LLMMessageStart{ID: "journey-m2"}
		out <- types.LLMTextDelta{Delta: journeyToolAnswer}
		out <- types.LLMMessageStop{StopReason: "stop"}
	}()
	return out, nil
}

// The journey: the user asks for something, the agent runs a shell
// command and answers. The frame shows the tool row and the answer.
// Later the user restarts, opens that session from the Sessions tab,
// and the tool row is still there: a resumed session reads like the
// live one did.
func TestJourneyToolTurnRendersAndResumes(t *testing.T) {
	journeyHome(t)
	journeyLocal(t)
	// Auto mode with shell access: tool calls run without a dialog (the
	// approval journey covers the dialog).
	t.Setenv("AH_EXECUTION_MODE", "yolo")
	t.Setenv("AH_PERM_EXECUTE", "true")

	app := bootJourneyApp(t)
	client := &toolTurnClient{}
	app.client = client
	app.loop = agent.NewLoop(client)
	tuiApp := wireJourneyTUI(t, app)
	screen := newJourneyScreen(t, tuiApp, 80, 24)
	// The user's own path: Chat tab, insert mode, type, Enter.
	screen.Keys("2")
	screen.Keys("i")
	screen.Submit(t, app, "say the needle")
	screen.waitFor(t, journeyToolAnswer)
	screen.waitIdle(t)

	if client.calls != 2 {
		t.Fatalf("model requests = %d, want 2 (tool call, then answer)", client.calls)
	}
	if !client.sawResult {
		t.Fatal("the follow-up request did not carry the shell output")
	}
	screen.AssertFitsWidth(80)
	screen.AssertContains("say the needle", "Shell", journeyToolCommand, journeyToolAnswer)
	sessionID := app.session.ID

	// Restart on the same home. The new boot starts a fresh chat, then
	// the user opens the earlier session from the Sessions tab.
	restarted := bootJourneyApp(t)
	if err := restarted.startFreshSession(restarted.session.Model); err != nil {
		t.Fatalf("start fresh session: %v", err)
	}
	if restarted.session.ID == sessionID {
		t.Fatal("restart did not move to a fresh session")
	}
	freshTUI := wireJourneyTUI(t, restarted)
	resumed := newJourneyScreen(t, freshTUI, 80, 24)
	resumed.AssertNotContains(journeyToolAnswer)

	(&tuiSessionsDelegate{app: restarted, tuiApp: freshTUI}).OnSessionSelect(sessionID)
	resumed.Pump()

	if restarted.session.ID != sessionID {
		t.Fatalf("active session = %s, want the resumed %s", restarted.session.ID, sessionID)
	}
	resumed.AssertFitsWidth(80)
	resumed.AssertContains("say the needle", "Shell", journeyToolCommand, journeyToolAnswer)
	assertSessionHasToolCall(t, restarted.session.Messages, "bash")
}

func assertSessionHasToolCall(t *testing.T, messages []types.Message, name string) {
	t.Helper()
	uses, results := 0, 0
	for _, m := range messages {
		for _, block := range m.Content {
			switch b := block.(type) {
			case types.ToolUseBlock:
				if b.Name == name {
					uses++
				}
			case types.ToolResultBlock:
				results++
			}
		}
	}
	if uses != 1 || results != 1 {
		t.Fatalf("resumed session has %d %s calls and %d results, want 1 and 1", uses, name, results)
	}
}
