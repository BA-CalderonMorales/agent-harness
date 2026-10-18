package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/audit"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/internal/testharness/testenv"
)

func TestToolProgressIsSanitizedTransientStatus(t *testing.T) {
	testenv.SetHome(t, t.TempDir())
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "local", AlwaysAllow: []string{"echo"}}, "test-model")
	logger, err := audit.NewLogger()
	if err != nil {
		t.Fatal(err)
	}
	app.auditLogger = logger
	tuiApp := tui.NewApp()
	app.tuiApp = tuiApp
	app.client = &burstClient{first: true, burstSize: 1}
	app.loop = agent.NewLoop(app.client)
	tool := echoTool()
	tool.Capabilities.IsEnabled = func() bool { return true }
	tool.Call = func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
		onProgress("\x1b[31mInspecting fixture\x1b[0m\nsource bytes")
		return tools.ToolResult{Data: "ok"}, nil
	}
	app.toolRegistry = tools.NewRegistry()
	app.toolRegistry.RegisterBuiltIn(tool)
	app.handleAgentLoopAsync("inspect", tuiApp)
	sawProgress := false
	for {
		msg := receiveTUIMessage(t, tuiApp)
		if status, ok := msg.(tui.StatusMsg); ok && strings.Contains(status.Text, "Inspecting fixture") {
			if status.Text != "Inspecting fixture source bytes" {
				t.Fatalf("unsafe progress display: %q", status.Text)
			}
			sawProgress = true
		}
		if failure, ok := msg.(tui.AgentErrorMsg); ok {
			t.Fatal(failure.Error)
		}
		if _, ok := msg.(tui.AgentDoneMsg); ok {
			break
		}
	}
	if !sawProgress {
		t.Fatal("tool progress never reached the TUI")
	}
	for _, msg := range app.session.Messages {
		data, err := json.Marshal(msg)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "Inspecting fixture") {
			t.Fatal("transient progress polluted durable session evidence")
		}
	}
}
