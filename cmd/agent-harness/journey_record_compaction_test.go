package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/record"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/pkg/messages"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

const (
	recordJourneyGoal       = "Migrate settings persistence to one write path."
	recordJourneyConstraint = "Never push directly to main."
	recordJourneyPending    = "add the regression test"
	recordJourneyOldRule    = "Always run the formatter first."
)

// capturingWindowClient wraps the 4096-window server fake and keeps every
// request, so the test can see what the model was actually sent.
type capturingWindowClient struct {
	*journeyWindowClient
	reqMu    sync.Mutex
	requests []llm.Request
}

func (c *capturingWindowClient) Stream(ctx context.Context, req llm.Request) (<-chan types.LLMEvent, error) {
	c.reqMu.Lock()
	c.requests = append(c.requests, req)
	c.reqMu.Unlock()
	return c.journeyWindowClient.Stream(ctx, req)
}

func journeyToolCall(id, name string, input map[string]any) types.Message {
	return types.Message{Role: types.RoleAssistant, Content: []types.ContentBlock{
		types.ToolUseBlock{ID: id, Name: name, Input: input},
	}}
}

func journeyToolResult(id, content string, isError bool) types.Message {
	return types.Message{Role: types.RoleUser, Content: []types.ContentBlock{
		types.ToolResultBlock{ToolUseID: id, Content: content, IsError: isError},
	}}
}

func journeyText(role types.MessageRole, text string) types.Message {
	return types.Message{Role: role, Content: []types.ContentBlock{types.TextBlock{Text: text}}}
}

// recordJourneySession is about three windows of tool-using history: a goal
// and a constraint, a todo plan with a pending item, writes, a failing test.
func recordJourneySession() []types.Message {
	msgs := []types.Message{
		journeyText(types.RoleUser, recordJourneyGoal+" "+recordJourneyConstraint),
		journeyToolCall("j1", "todo_write", map[string]any{"todos": []any{
			map[string]any{"text": "unify the writers", "status": "done"},
			map[string]any{"text": recordJourneyPending, "status": "pending"},
		}}),
		journeyToolResult("j1", "ok", false),
	}
	for i := 0; i < 4; i++ {
		id := fmt.Sprintf("jw%d", i)
		msgs = append(msgs, journeyToolCall(id, "write", map[string]any{"file_path": fmt.Sprintf("internal/core/state/writer_%d.go", i)}),
			journeyToolResult(id, "wrote", false))
	}
	msgs = append(msgs, journeyToolCall("jb", "bash", map[string]any{"command": "go test ./internal/core/state/..."}),
		journeyToolResult("jb", "FAIL: settings writer still races on save", true))
	filler := strings.Repeat("progress ", 45)
	for messages.EstimateTokens(msgs) < 3*journeyWindow {
		msgs = append(msgs, journeyText(types.RoleUser, "next"), journeyText(types.RoleAssistant, filler))
	}
	return msgs
}

// The journey: a long tool-using session on a 4096-token server comes back
// for one more turn. The turn completes with no summarizer request, the
// model is told the goal, and the session file's record still holds the
// full goal and todo plan after the history shrank.
func TestJourneyRecordCompactionOnSmallWindow(t *testing.T) {
	workspace := journeyHome(t)
	journeyLocal(t)

	app := bootJourneyApp(t)
	history := recordJourneySession()
	app.session.Messages = history
	// An earlier compaction already dropped the turn that stated this
	// rule; only the session's record still knows it.
	app.session.Record = record.Fold(history)
	app.session.Record.Constraints = append(app.session.Record.Constraints, recordJourneyOldRule)
	app.sessionManager.SetCurrent(app.session)
	if _, err := app.sessionManager.SaveCurrent(); err != nil {
		t.Fatalf("save long session: %v", err)
	}

	client := &capturingWindowClient{journeyWindowClient: &journeyWindowClient{}}
	app.client = client
	app.loop.Client = client
	tuiApp := wireJourneyTUI(t, app)
	screen := newJourneyScreen(t, tuiApp, 80, 24)
	screen.SendAndPump(tui.ModelContextMsg{Provider: app.config.Provider, Model: app.session.Model, ContextLength: journeyWindow})

	screen.Keys("2")
	screen.Keys("i")
	screen.Submit(t, app, "continue")
	screen.waitFor(t, journeyAnswerText)
	screen.waitIdle(t)

	if client.summaries != 0 {
		t.Fatalf("summarizer requests = %d, want 0", client.summaries)
	}
	if client.refused != 0 {
		t.Fatalf("the server refused %d request(s)", client.refused)
	}
	client.reqMu.Lock()
	first := client.requests[0]
	client.reqMu.Unlock()
	if !messagesMention(first.Messages, recordJourneyGoal) || !messagesMention(first.Messages, recordJourneyPending) {
		t.Fatal("the model's first request lost the goal or the pending todo")
	}
	if !messagesMention(first.Messages, recordJourneyOldRule) {
		t.Fatal("the model's first request lost a rule only the session record held")
	}
	screen.AssertContains("Context compacted")
	if len(app.session.Messages) >= len(history) {
		t.Fatalf("session holds %d messages, want fewer than %d", len(app.session.Messages), len(history))
	}

	files := sessionFilesUnder(t, filepath.Dir(workspace))
	if len(files) != 1 {
		t.Fatalf("session files = %v", files)
	}
	// The file is append-only with the last record line winning. The
	// compaction rewrite leaves one line and the turn's reply may append
	// one more, so what must hold is that every line is the full record,
	// never a record shrunk along with the history.
	lines := recordLinesIn(t, files[0])
	if len(lines) == 0 || len(lines) > 2 {
		t.Fatalf("record lines = %d, want 1 or 2", len(lines))
	}
	for _, line := range lines {
		if !strings.Contains(line, recordJourneyGoal) || !strings.Contains(line, recordJourneyPending) {
			t.Fatalf("a record line lost the goal or the plan: %s", line)
		}
	}
	saved := lastRecordIn(t, files[0])
	if !strings.HasPrefix(saved.Goal, recordJourneyGoal) || len(saved.Constraints) != 2 {
		t.Fatalf("saved goal/constraints = %q / %v, want the full originals", saved.Goal, saved.Constraints)
	}
	if len(saved.Todos) != 2 || saved.Todos[1].Text != recordJourneyPending {
		t.Fatalf("saved todos = %+v, want the full plan", saved.Todos)
	}
}
