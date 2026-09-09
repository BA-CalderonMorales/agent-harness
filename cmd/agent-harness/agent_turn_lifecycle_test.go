package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

type terminalErrorClient struct{ llm.Client }

func (terminalErrorClient) Stream(context.Context, llm.Request) (<-chan types.LLMEvent, error) {
	return nil, errors.New("provider disconnected")
}

func TestRequestCommandApprovalFollowsTurnCancellation(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "local"}, "test-model")
	app.tuiApp = tui.NewApp()
	ctx, cancel := context.WithCancel(context.Background())
	beginApprovalTurn(app, ctx)
	defer endApprovalTurn(app)

	result := make(chan error, 1)
	go func() {
		_, _, err := app.requestCommandApproval("bash", "echo test", nil)
		result <- err
	}()
	// Wait until the request has crossed the real TUI message channel.
	_ = receiveTUIMessage(t, app.tuiApp)
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("approval cancellation error = %v, want context.Canceled", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("approval remained blocked after turn cancellation")
	}
}

func TestRunAgentTurnDoesNotCompleteAfterProviderError(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "local"}, "test-model")
	app.tuiApp = tui.NewApp()
	app.client = terminalErrorClient{}
	app.loop = agent.NewLoop(app.client)
	app.toolRegistry = tools.NewRegistry()

	go app.runAgentTurn("fail", app.tuiApp)
	var errorsSeen, doneSeen int
	deadline := time.After(time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("provider error did not settle")
		default:
		}
		msg := receiveTUIMessage(t, app.tuiApp)
		switch msg.(type) {
		case tui.AgentErrorMsg:
			errorsSeen++
		case tui.AgentDoneMsg:
			doneSeen++
		}
		if errorsSeen > 0 {
			goto settled
		}
	}
settled:
	if doneSeen != 0 {
		t.Fatalf("AgentDone count = %d after provider error", doneSeen)
	}
}
