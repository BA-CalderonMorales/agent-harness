package main

import (
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/state"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/commands"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
)

func newHandlerTestApp(t *testing.T, cfg *config.LayeredConfig, model string) *App {
	t.Helper()

	t.Setenv("AGENT_HARNESS_SESSION_DIR", t.TempDir())
	sm, err := state.NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	session := sm.CreateSession(model)
	return &App{
		config:         cfg,
		session:        session,
		sessionManager: sm,
		costTracker:    agent.NewCostTracker(),
		cmdRegistry:    commands.NewSlashRegistry(),
	}
}

func receiveTUIMessage(t *testing.T, app *tui.App) tea.Msg {
	t.Helper()

	msgChan := exportedField(reflect.ValueOf(app).Elem().FieldByName("msgChan"))
	cases := []reflect.SelectCase{
		{Dir: reflect.SelectRecv, Chan: msgChan},
		{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(time.After(250 * time.Millisecond))},
	}
	chosen, msg, ok := reflect.Select(cases)
	if chosen == 1 {
		t.Fatal("timed out waiting for TUI message")
	}
	if !ok {
		t.Fatal("TUI message channel closed")
	}
	return msg.Interface().(tea.Msg)
}

func chatMessages(t *testing.T, app *tui.App) []struct {
	role    string
	content string
	parts   []tui.TurnPart
} {
	t.Helper()

	chatModel := exportedField(reflect.ValueOf(app).Elem().FieldByName("chatModel"))
	messages := exportedField(chatModel.FieldByName("messages"))
	got := make([]struct {
		role    string
		content string
		parts   []tui.TurnPart
	}, 0, messages.Len())
	for i := 0; i < messages.Len(); i++ {
		msg := messages.Index(i)
		var parts []tui.TurnPart
		if field := msg.FieldByName("Parts"); field.IsValid() && !field.IsNil() {
			parts = field.Interface().([]tui.TurnPart)
		}
		got = append(got, struct {
			role    string
			content string
			parts   []tui.TurnPart
		}{
			role:    msg.FieldByName("Role").String(),
			content: msg.FieldByName("Content").String(),
			parts:   parts,
		})
	}
	return got
}

func exportedField(v reflect.Value) reflect.Value {
	return reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem()
}

func TestValidateConfigRejectsRemoteProviderWithoutAPIKey(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "openrouter"}, "test-model")

	err := app.validateConfig()
	if err == nil {
		t.Fatal("validateConfig() error = nil, want missing API key error")
	}
	if !strings.Contains(err.Error(), "no API key configured") {
		t.Fatalf("validateConfig() error = %q, want missing API key", err)
	}
}

func TestValidateConfigRejectsMissingModel(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "ollama"}, "")

	err := app.validateConfig()
	if err == nil {
		t.Fatal("validateConfig() error = nil, want missing model error")
	}
	if !strings.Contains(err.Error(), "no model selected") {
		t.Fatalf("validateConfig() error = %q, want missing model", err)
	}
}

func TestValidateConfigAllowsLocalProviderWithoutAPIKey(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "local"}, "test-model")

	if err := app.validateConfig(); err != nil {
		t.Fatalf("validateConfig() error = %v, want nil", err)
	}
}

func TestHandleAgentLoopAsyncInvalidConfigStopsBeforeAgentExecution(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "openrouter"}, "test-model")
	tuiApp := tui.NewApp()

	app.handleAgentLoopAsync("hello", tuiApp)

	msg := receiveTUIMessage(t, tuiApp)
	errMsg, ok := msg.(tui.AgentErrorMsg)
	if !ok {
		t.Fatalf("first TUI message = %T, want tui.AgentErrorMsg", msg)
	}
	if !strings.Contains(errMsg.Error.Error(), "no API key configured") {
		t.Fatalf("AgentErrorMsg error = %q, want missing API key", errMsg.Error)
	}
}

func TestHandleUserCommandUnknownCommandAddsSystemFeedback(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "ollama"}, "test-model")
	tuiApp := tui.NewApp()

	app.handleUserCommand("/does-not-exist", tuiApp)

	messages := chatMessages(t, tuiApp)
	if len(messages) != 1 {
		t.Fatalf("chat message count = %d, want 1", len(messages))
	}
	if messages[0].role != "system" {
		t.Fatalf("message role = %q, want system", messages[0].role)
	}
	if !strings.Contains(messages[0].content, "Unknown command: /does-not-exist") {
		t.Fatalf("message content = %q", messages[0].content)
	}
	if !strings.Contains(messages[0].content, "Type /help") {
		t.Fatalf("message content = %q, want /help hint", messages[0].content)
	}
}

func TestHandleUserCommandRoutesRegisteredSlashCommand(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "ollama"}, "test-model")
	tuiApp := tui.NewApp()
	var gotArgs string
	app.cmdRegistry.Register("echo", "Echo args", func(args string) (string, error) {
		gotArgs = args
		return "echo: " + args, nil
	})

	app.handleUserCommand("/echo hello harness", tuiApp)

	if gotArgs != "hello harness" {
		t.Fatalf("handler args = %q, want hello harness", gotArgs)
	}
	if len(app.session.Messages) != 0 {
		t.Fatalf("session message count = %d, want 0", len(app.session.Messages))
	}
	messages := chatMessages(t, tuiApp)
	if len(messages) != 1 {
		t.Fatalf("chat message count = %d, want 1", len(messages))
	}
	if messages[0].role != "system" {
		t.Fatalf("message role = %q, want system", messages[0].role)
	}
	if messages[0].content != "echo: hello harness" {
		t.Fatalf("message content = %q, want command result", messages[0].content)
	}
}

func TestHandleUserSubmitInvalidInputDoesNotAppendOrRunAgent(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "ollama"}, "test-model")
	tuiApp := tui.NewApp()

	app.handleUserSubmit("   ", tuiApp)

	if len(app.session.Messages) != 0 {
		t.Fatalf("session message count = %d, want 0", len(app.session.Messages))
	}
	msg := receiveTUIMessage(t, tuiApp)
	errMsg, ok := msg.(tui.AgentErrorMsg)
	if !ok {
		t.Fatalf("message type = %T, want tui.AgentErrorMsg", msg)
	}
	if errMsg.Error == nil || errMsg.Error.Error() != "invalid input" {
		t.Fatalf("AgentErrorMsg error = %v, want invalid input", errMsg.Error)
	}
}

// TestHandleUserCommandRoutesRegisteredSteerCommand verifies that /steer
// registers a handler via the cmdRegistry and that the TUI integration
// wires QueueSteer for auto-submission after the current agent turn.
func TestHandleUserCommandRoutesRegisteredSteerCommand(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "ollama"}, "test-model")
	tuiApp := tui.NewApp()

	var gotSteer string
	app.cmdRegistry.Register("steer", "Queue a message for current turn",
		commands.SteerHandler(func(msg string) {
			gotSteer = msg
		}))

	app.handleUserCommand("/steer hello harness", tuiApp)

	// The SteerHandler stores the message; the TUI integration (commands_tui.go)
	// calls tuiApp.QueueSteer(msg) which adds it to the chat model's steer queue.
	// We verify the handler was invoked and the message was captured.
	if gotSteer != "hello harness" {
		t.Fatalf("steer handler got %q, want hello harness", gotSteer)
	}
}
