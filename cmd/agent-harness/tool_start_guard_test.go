package main

import (
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools/builtin"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// TestToolUseStartToleratesOptionalCallbacks pins the nil-deref guard:
// tools may omit UserFacingName / GetActivityDescription (web_fetch,
// ask, plan, todo, … did), and the start event must fall back to the
// raw name instead of panicking the turn (session 4d06abe0 died on the
// model's very first web_fetch call).
func TestToolUseStartToleratesOptionalCallbacks(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "local"}, "test-model")
	app.toolRegistry = tools.NewRegistry()
	app.toolRegistry.RegisterBuiltIn(builtin.WebFetchTool) // no UserFacingName-less path

	tuiApp := tui.NewApp()
	app.tuiApp = tuiApp

	app.handleToolUseStart(types.ToolUseBlock{
		ID:    "call-1",
		Name:  "web_fetch",
		Input: map[string]any{"url": "https://example.com"},
	}, tuiApp)

	msg := receiveTUIMessage(t, tuiApp)
	start, ok := msg.(tui.AgentToolStartMsg)
	if !ok {
		t.Fatalf("expected AgentToolStartMsg, got %T", msg)
	}
	if start.DisplayName != "web_fetch" || start.ActivityDesc != "Fetching https://example.com" {
		t.Fatalf("start event = %+v", start)
	}
}
