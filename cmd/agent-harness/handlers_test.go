package main

import (
	"os"
	"path/filepath"
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

func TestImproveCommandRunsDeterministicWorkflow(t *testing.T) {
	root := t.TempDir()
	writeHandlerTestFile(t, root, "go.mod", "module example.com/selfimprove\n\ngo 1.22\n")
	writeHandlerTestFile(t, root, "selfimprove_test.go", "package selfimprove\n\nimport \"testing\"\n\nfunc TestSelfImprove(t *testing.T) {}\n")
	writeHandlerTestFile(t, root, "plans/agent-harness/PLAN.md", "# Agent Harness Plan Index\n\n## Current Domain\n\n- Date: `2026-06-13`\n")
	writeHandlerTestFile(t, root, "plans/agent-harness/2026-06-13/GOAL.md", "# 2026-06-13 Goal\n\n## Today\n\n- [ ] Resume useful context.\n")
	writeHandlerTestFile(t, root, "plans/agent-harness/2026-06-13/PLAN.md", "# 2026-06-13 Plan\n\n## Today\n\n- [ ] Implement the next bounded slice.\n")

	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "ollama"}, "test-model")
	app.cwd = root
	app.initCommands()

	result, handled, err := app.cmdRegistry.Handle("/improve")
	if err != nil {
		t.Fatalf("/improve error = %v\n%s", err, result)
	}
	if !handled {
		t.Fatal("/improve was not handled")
	}
	if !strings.Contains(result, "Self-improvement workflow: passed") {
		t.Fatalf("result missing passed summary:\n%s", result)
	}
	if !strings.Contains(result, "Next action: Implement the next bounded slice.") {
		t.Fatalf("result missing next action:\n%s", result)
	}
	if _, err := os.Stat(filepath.Join(root, "plans", "agent-harness", "2026-06-14", "GOAL.md")); err != nil {
		t.Fatalf("next GOAL.md not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "plans", "agent-harness", "2026-06-14", "PLAN.md")); err != nil {
		t.Fatalf("next PLAN.md not created: %v", err)
	}
}

func writeHandlerTestFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
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
} {
	t.Helper()

	chatModel := exportedField(reflect.ValueOf(app).Elem().FieldByName("chatModel"))
	messages := exportedField(chatModel.FieldByName("messages"))
	got := make([]struct {
		role    string
		content string
	}, 0, messages.Len())
	for i := 0; i < messages.Len(); i++ {
		msg := messages.Index(i)
		got = append(got, struct {
			role    string
			content string
		}{
			role:    msg.FieldByName("Role").String(),
			content: msg.FieldByName("Content").String(),
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

func TestHandleUserSubmitLoginStepDoesNotAppendUserMessage(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{}, "test-model")
	app.loginState = loginProvider
	tuiApp := tui.NewApp()

	app.handleUserSubmit("ollama", tuiApp)

	if len(app.session.Messages) != 0 {
		t.Fatalf("session message count = %d, want 0", len(app.session.Messages))
	}
	if app.config.Provider != "ollama" {
		t.Fatalf("provider = %q, want ollama", app.config.Provider)
	}
	if app.config.APIKey != "ollama" {
		t.Fatalf("API key = %q, want ollama sentinel", app.config.APIKey)
	}
	if app.loginState != loginModel {
		t.Fatalf("loginState = %v, want loginModel", app.loginState)
	}
}
