package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

type durableCompactionClient struct {
	mu             sync.Mutex
	requests       []llm.Request
	mainCalls      int
	firstMainError error
}

func (c *durableCompactionClient) Stream(_ context.Context, req llm.Request) (<-chan types.LLMEvent, error) {
	c.mu.Lock()
	c.requests = append(c.requests, req)
	isSummary := strings.Contains(req.SystemPrompt, "context summarizer")
	if !isSummary {
		c.mainCalls++
	}
	mainCall := c.mainCalls
	c.mu.Unlock()

	text := "assistant response"
	var events []types.LLMEvent
	switch {
	case isSummary:
		text = "ORIGINAL-GOAL, CONSTRAINT, PENDING-WORK, TOOL-RESULT, persona, and plan state preserved"
		events = durableCompactionTextEvents(text)
	case mainCall == 1 && c.firstMainError != nil:
		events = []types.LLMEvent{types.LLMError{Error: c.firstMainError}}
	default:
		events = durableCompactionTextEvents(text)
	}

	out := make(chan types.LLMEvent, len(events))
	for _, event := range events {
		out <- event
	}
	close(out)
	return out, nil
}

func (c *durableCompactionClient) recordedRequests() []llm.Request {
	c.mu.Lock()
	defer c.mu.Unlock()
	requests := make([]llm.Request, len(c.requests))
	copy(requests, c.requests)
	return requests
}

func durableCompactionTextEvents(text string) []types.LLMEvent {
	return []types.LLMEvent{
		types.LLMMessageStart{ID: "durable-compaction"},
		types.LLMTextDelta{Delta: text},
		types.LLMMessageStop{StopReason: "stop", Model: "test-model"},
	}
}

func TestAutomaticCompactionPersistsTheContextUsedByTheModel(t *testing.T) {
	cfg := &config.LayeredConfig{
		Provider:       "local",
		Model:          "test-model",
		PermissionMode: config.PermissionReadOnly,
		PermRead:       true,
		PermExplicit:   true,
	}
	app := newHandlerTestApp(t, cfg, "test-model")
	app.cwd = t.TempDir()
	app.session.Persona = "scientist"
	app.session.PlanMode = true
	app.session.Messages = durableCompactionMessages(25)
	app.sessionManager.SetCurrent(app.session)
	app.initTools()

	client := &durableCompactionClient{}
	app.client = client
	app.loop = agent.NewLoop(client)
	app.loop.Config.BlockingTokenLimit = 100
	app.loop.Config.DefaultMaxTurns = 1

	app.handleAgentLoopAsync("continue", tui.NewApp())

	if len(app.session.Messages) >= 25 {
		t.Fatalf("session retained %d messages after automatic compaction, want fewer than original 25", len(app.session.Messages))
	}
	if !durableMessagesContain(app.session.Messages, "ORIGINAL-GOAL") {
		t.Fatal("active session does not persist the goal-preserving compaction summary")
	}
	if app.session.Persona != "scientist" || !app.session.PlanMode {
		t.Fatalf("active session lost persona/plan state: persona=%q plan=%v", app.session.Persona, app.session.PlanMode)
	}

	persistedPath := app.sessionManager.GetDefaultSessionPath()
	persisted, err := app.sessionManager.ReadSession(app.session.ID)
	if err != nil {
		t.Fatalf("ReadSession(%q) error = %v", persistedPath, err)
	}
	if len(persisted.Messages) != len(app.session.Messages) {
		t.Fatalf("persisted message count = %d, active count = %d", len(persisted.Messages), len(app.session.Messages))
	}
	if !durableMessagesContain(persisted.Messages, "ORIGINAL-GOAL") {
		t.Fatal("persisted session does not contain the goal-preserving compaction summary")
	}
	if persisted.Persona != "scientist" || !persisted.PlanMode {
		t.Fatalf("persisted session lost persona/plan state: persona=%q plan=%v", persisted.Persona, persisted.PlanMode)
	}

	mainRequests := durableMainRequests(client.recordedRequests())
	if len(mainRequests) != 1 {
		t.Fatalf("main request count = %d, want 1", len(mainRequests))
	}
	assertPersistedRequestPrefix(t, persisted.Messages, mainRequests[0].Messages)
}

func TestReactiveCompactionPersistsTheRetryContext(t *testing.T) {
	cfg := &config.LayeredConfig{
		Provider:       "local",
		Model:          "test-model",
		PermissionMode: config.PermissionReadOnly,
		PermRead:       true,
		PermExplicit:   true,
	}
	app := newHandlerTestApp(t, cfg, "test-model")
	app.cwd = t.TempDir()
	app.session.Persona = "scientist"
	app.session.PlanMode = true
	app.session.Messages = durableCompactionMessages(25)
	app.sessionManager.SetCurrent(app.session)
	app.initTools()

	client := &durableCompactionClient{
		firstMainError: fmt.Errorf("prompt_too_long: context length exceeded"),
	}
	app.client = client
	app.loop = agent.NewLoop(client)
	app.loop.Config.AutoCompactEnabled = false
	app.loop.Config.BlockingTokenLimit = 100
	app.loop.Config.DefaultMaxTurns = 1

	app.handleAgentLoopAsync("continue", tui.NewApp())

	mainRequests := durableMainRequests(client.recordedRequests())
	if len(mainRequests) != 2 {
		t.Fatalf("main request count = %d, want initial request and compacted retry", len(mainRequests))
	}
	if len(mainRequests[1].Messages) >= len(mainRequests[0].Messages) {
		t.Fatalf("retry message count = %d, want fewer than initial %d", len(mainRequests[1].Messages), len(mainRequests[0].Messages))
	}

	persisted, err := app.sessionManager.ReadSession(app.session.ID)
	if err != nil {
		t.Fatalf("LoadSession() error = %v", err)
	}
	assertPersistedRequestPrefix(t, persisted.Messages, mainRequests[1].Messages)
	if !durableMessagesContain(persisted.Messages, "ORIGINAL-GOAL") {
		t.Fatal("reactively compacted session lost the original goal")
	}
	if persisted.Persona != "scientist" || !persisted.PlanMode {
		t.Fatalf("reactive compaction lost persona/plan state: persona=%q plan=%v", persisted.Persona, persisted.PlanMode)
	}
}

func TestCompactionPersistenceFailureDoesNotReportSuccess(t *testing.T) {
	cfg := &config.LayeredConfig{
		Provider:       "local",
		Model:          "test-model",
		PermissionMode: config.PermissionReadOnly,
		PermRead:       true,
		PermExplicit:   true,
	}
	app := newHandlerTestApp(t, cfg, "test-model")
	app.cwd = t.TempDir()
	app.session.Messages = durableCompactionMessages(25)
	app.sessionManager.SetCurrent(app.session)
	app.initTools()

	client := &durableCompactionClient{}
	app.client = client
	app.loop = agent.NewLoop(client)
	app.loop.Config.BlockingTokenLimit = 100
	app.loop.Config.DefaultMaxTurns = 1

	// Block persistence at the project directory level: replace it with
	// a file so MkdirAll/OpenFile cannot create the session store.
	projectDir := filepath.Dir(app.sessionManager.GetDefaultSessionPath())
	if err := os.RemoveAll(projectDir); err != nil {
		t.Fatalf("remove temporary project sessions directory: %v", err)
	}
	if err := os.WriteFile(projectDir, []byte("blocks session persistence"), 0o600); err != nil {
		t.Fatalf("replace temporary project directory with file: %v", err)
	}

	tuiApp := tui.NewApp()
	app.handleAgentLoopAsync("continue", tuiApp)

	var sawPersistenceError bool
	for _, message := range drainCompactionTUIMessages(t, tuiApp) {
		switch value := message.(type) {
		case tui.AgentErrorMsg:
			if value.Error != nil && strings.Contains(value.Error.Error(), "persist compacted session") {
				sawPersistenceError = true
			}
		case tui.AgentDoneMsg:
			t.Fatal("compaction persistence failure was followed by AgentDoneMsg")
		}
	}
	if !sawPersistenceError {
		t.Fatal("compaction persistence failure was not surfaced to the TUI")
	}
}

func durableMainRequests(requests []llm.Request) []llm.Request {
	mainRequests := make([]llm.Request, 0, len(requests))
	for _, request := range requests {
		if !strings.Contains(request.SystemPrompt, "context summarizer") {
			mainRequests = append(mainRequests, request)
		}
	}
	return mainRequests
}

func assertPersistedRequestPrefix(t *testing.T, persisted, requested []types.Message) {
	t.Helper()
	if len(persisted) < len(requested) {
		t.Fatalf("persisted messages = %d, want at least request prefix %d", len(persisted), len(requested))
	}
	want, err := json.Marshal(requested)
	if err != nil {
		t.Fatalf("marshal requested context: %v", err)
	}
	got, err := json.Marshal(persisted[:len(requested)])
	if err != nil {
		t.Fatalf("marshal persisted context prefix: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("persisted context prefix differs from model request:\n got: %s\nwant: %s", got, want)
	}
}

func drainCompactionTUIMessages(t *testing.T, app *tui.App) []any {
	t.Helper()
	channel := exportedField(reflect.ValueOf(app).Elem().FieldByName("msgChan"))
	var messages []any
	for {
		chosen, message, ok := reflect.Select([]reflect.SelectCase{
			{Dir: reflect.SelectRecv, Chan: channel},
			{Dir: reflect.SelectDefault},
		})
		if chosen == 1 {
			return messages
		}
		if !ok {
			return messages
		}
		messages = append(messages, message.Interface())
	}
}

func durableCompactionMessages(count int) []types.Message {
	messages := make([]types.Message, 0, count)
	for i := 0; i < count; i++ {
		text := "ordinary history that consumes enough tokens to force bounded compaction"
		switch i {
		case 0:
			text = "ORIGINAL-GOAL: stabilize the production loop"
		case 1:
			text = "CONSTRAINT: preserve the canonical loop"
		case 2:
			text = "PENDING-WORK: permissions, executor, SSE, compaction"
		}
		message := types.Message{
			UUID:    sprintf("durable-%02d", i),
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.TextBlock{Text: text}},
		}
		if i == 3 {
			message.Content = []types.ContentBlock{types.ToolResultBlock{
				ToolUseID: "tool-1",
				Content:   "TOOL-RESULT: race reproduced",
			}}
		}
		messages = append(messages, message)
	}
	return messages
}

func durableMessagesContain(messages []types.Message, want string) bool {
	for _, message := range messages {
		for _, block := range message.Content {
			switch value := block.(type) {
			case types.TextBlock:
				if strings.Contains(value.Text, want) {
					return true
				}
			case types.ToolResultBlock:
				if strings.Contains(value.Content, want) {
					return true
				}
			}
		}
	}
	return false
}
