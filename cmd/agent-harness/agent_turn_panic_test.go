package main

import (
	"context"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// panickingClient explodes synchronously on Stream, like the corrupted
// renderer / race states that took the whole TUI down with it.
type panickingClient struct{ llm.Client }

func (c *panickingClient) Stream(ctx context.Context, req llm.Request) (<-chan types.LLMEvent, error) {
	panic("boom: turn panic must cost the turn, not the program")
}

// TestTurnPanicRecoversAsErrorMessage pins the goroutine panic guard:
// the agent turn runs outside the TUI's recover nets, so a panic in the
// loop used to bubble to bubbletea's goroutine handler and kill the
// program ("program was killed: program experienced a panic"). A
// panicking turn must degrade to an error message instead.
func TestTurnPanicRecoversAsErrorMessage(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "local"}, "test-model")
	tuiApp := tui.NewApp()
	app.tuiApp = tuiApp
	app.loop = agent.NewLoop(&panickingClient{})
	app.toolRegistry = tools.NewRegistry()

	// Before the fix this call panicked the test binary.
	app.handleAgentLoopAsync("search the web", tuiApp)

	var model tea.Model = tuiApp
	sawError, sawDone := false, false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		msg := receiveTUIMessage(t, tuiApp)
		model, _ = model.Update(msg)
		switch m := msg.(type) {
		case tui.AgentErrorMsg:
			sawError = true
			if m.Error == nil || m.Error.Error() == "" {
				t.Fatal("AgentErrorMsg carries no explanation")
			}
		case tui.AgentDoneMsg:
			sawDone = true
		}
		if sawError && sawDone {
			return
		}
	}
	t.Fatalf("turn panic not surfaced: sawError=%v sawDone=%v", sawError, sawDone)
}
