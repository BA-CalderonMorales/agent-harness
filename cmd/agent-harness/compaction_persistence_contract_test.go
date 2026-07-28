package main

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/state"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

type durableCompactionClient struct {
	mu       sync.Mutex
	requests []llm.Request
}

func (c *durableCompactionClient) Stream(_ context.Context, req llm.Request) (<-chan types.LLMEvent, error) {
	c.mu.Lock()
	c.requests = append(c.requests, req)
	c.mu.Unlock()

	text := "assistant response"
	if strings.Contains(req.SystemPrompt, "context summarizer") {
		text = "ORIGINAL-GOAL, CONSTRAINT, PENDING-WORK, TOOL-RESULT, persona, and plan state preserved"
	}
	events := []types.LLMEvent{
		types.LLMMessageStart{ID: "durable-compaction"},
		types.LLMTextDelta{Delta: text},
		types.LLMMessageStop{StopReason: "stop", Model: "test-model"},
	}
	out := make(chan types.LLMEvent, len(events))
	for _, event := range events {
		out <- event
	}
	close(out)
	return out, nil
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
	persisted, err := state.LoadSession(persistedPath)
	if err != nil {
		t.Fatalf("LoadSession(%q) error = %v", persistedPath, err)
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
