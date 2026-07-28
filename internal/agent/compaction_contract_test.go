package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

type compactionContractClient struct {
	mu                sync.Mutex
	requests          []llm.Request
	mainCalls         int
	firstMainError    error
	compactionSummary string
	mainResponse      string
}

func (c *compactionContractClient) Stream(_ context.Context, req llm.Request) (<-chan types.LLMEvent, error) {
	c.mu.Lock()
	c.requests = append(c.requests, req)
	isSummary := strings.Contains(req.SystemPrompt, "context summarizer")
	if !isSummary {
		c.mainCalls++
	}
	mainCall := c.mainCalls
	c.mu.Unlock()

	var events []types.LLMEvent
	switch {
	case isSummary:
		events = compactionTextEvents(c.compactionSummary)
	case mainCall == 1 && c.firstMainError != nil:
		events = []types.LLMEvent{types.LLMError{Error: c.firstMainError}}
	default:
		events = compactionTextEvents(c.mainResponse)
	}

	out := make(chan types.LLMEvent, len(events))
	for _, event := range events {
		out <- event
	}
	close(out)
	return out, nil
}

func (c *compactionContractClient) recordedRequests() []llm.Request {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]llm.Request, len(c.requests))
	copy(out, c.requests)
	return out
}

func compactionTextEvents(text string) []types.LLMEvent {
	return []types.LLMEvent{
		types.LLMMessageStart{ID: "compaction-contract"},
		types.LLMTextDelta{Delta: text},
		types.LLMMessageStop{StopReason: "stop", Model: "test-model"},
	}
}

func compactionMessages(count int) []types.Message {
	messages := make([]types.Message, 0, count)
	for i := 0; i < count; i++ {
		text := fmt.Sprintf("history-%02d: ordinary context that consumes tokens", i)
		if i == 0 {
			text = "ORIGINAL-GOAL: stabilize the production loop; CONSTRAINT: preserve permissions"
		}
		if i == 1 {
			text = "PENDING-WORK: executor, SSE, and compaction; PLAN: characterize then repair"
		}
		messages = append(messages, types.Message{
			UUID:    fmt.Sprintf("message-%02d", i),
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.TextBlock{Text: text}},
		})
	}
	return messages
}

func TestAutoCompactionSummarizesRemovedPrefixAndAffectsCurrentRequest(t *testing.T) {
	const summary = "ORIGINAL-GOAL preserved; CONSTRAINT preserved; PENDING-WORK and PLAN preserved"
	client := &compactionContractClient{
		compactionSummary: summary,
		mainResponse:      "done",
	}
	loop := NewLoop(client)
	loop.Config.BlockingTokenLimit = 100
	loop.Config.DefaultMaxTurns = 1

	original := compactionMessages(25)
	params := edgeParams(nil)
	params.Messages = original
	state := edgeLoopState(original)
	out := make(chan types.StreamEvent, 64)

	terminal := loop.queryLoop(context.Background(), params, state, out)
	if terminal.Reason != TerminalReasonComplete {
		t.Fatalf("terminal reason = %q, want %q (error: %v)", terminal.Reason, TerminalReasonComplete, terminal.Error)
	}

	requests := client.recordedRequests()
	if len(requests) != 2 {
		t.Fatalf("request count = %d, want one summary request and one main request", len(requests))
	}

	summaryPrompt := firstText(requests[0].Messages)
	if !strings.Contains(summaryPrompt, "ORIGINAL-GOAL") {
		t.Fatalf("summarizer did not receive the removed prefix:\n%s", summaryPrompt)
	}

	mainRequest := requests[1]
	if len(mainRequest.Messages) >= len(original) {
		t.Fatalf("current main request kept %d messages, want fewer than original %d", len(mainRequest.Messages), len(original))
	}
	if !messagesContainText(mainRequest.Messages, summary) {
		t.Fatalf("current main request does not contain the compaction summary: %#v", mainRequest.Messages)
	}
}

func TestAutoCompactionDoesNotPanicForLongHistory(t *testing.T) {
	loop := NewLoop(&compactionContractClient{
		compactionSummary: "long history summarized",
		mainResponse:      "unused",
	})
	loop.Config.BlockingTokenLimit = 100
	state := edgeLoopState(compactionMessages(60))

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("autoCompactMessages panicked for long history: %v", recovered)
		}
	}()

	if notice := loop.autoCompactMessages(state); notice == "" {
		t.Fatal("autoCompactMessages returned an empty notice for oversized history")
	}
	if len(state.messages) >= 60 {
		t.Fatalf("message count after compaction = %d, want fewer than 60", len(state.messages))
	}
}

func TestPromptTooLongRecoveryActuallyCompactsBeforeRetry(t *testing.T) {
	client := &compactionContractClient{
		firstMainError:    fmt.Errorf("prompt_too_long: context length exceeded"),
		compactionSummary: "ORIGINAL-GOAL, constraints, pending work, and plan preserved",
		mainResponse:      "recovered",
	}
	loop := NewLoop(client)
	loop.Config.AutoCompactEnabled = false
	loop.Config.BlockingTokenLimit = 100
	loop.Config.DefaultMaxTurns = 1

	original := compactionMessages(25)
	params := edgeParams(nil)
	params.Messages = original
	state := edgeLoopState(original)
	out := make(chan types.StreamEvent, 64)

	terminal := loop.queryLoop(context.Background(), params, state, out)
	if terminal.Reason != TerminalReasonComplete {
		t.Fatalf("terminal reason = %q, want %q (error: %v)", terminal.Reason, TerminalReasonComplete, terminal.Error)
	}

	var mainRequests []llm.Request
	for _, request := range client.recordedRequests() {
		if !strings.Contains(request.SystemPrompt, "context summarizer") {
			mainRequests = append(mainRequests, request)
		}
	}
	if len(mainRequests) != 2 {
		t.Fatalf("main request count = %d, want initial request plus one retry", len(mainRequests))
	}
	if len(mainRequests[1].Messages) >= len(mainRequests[0].Messages) {
		t.Fatalf("retry message count = %d, want fewer than initial %d", len(mainRequests[1].Messages), len(mainRequests[0].Messages))
	}
	if !messagesContainText(mainRequests[1].Messages, "ORIGINAL-GOAL") {
		t.Fatal("retry request lost the original goal")
	}
}

func TestEstimateTokensCoversEveryContentBlockType(t *testing.T) {
	message := types.Message{
		Role: types.RoleAssistant,
		Content: []types.ContentBlock{
			types.TextBlock{Text: strings.Repeat("t", 40)},
			types.ThinkingBlock{Thinking: strings.Repeat("h", 400), Signature: strings.Repeat("s", 40)},
			types.ToolUseBlock{ID: "call-1", Name: "tool", Input: map[string]any{"value": strings.Repeat("i", 80)}},
			types.ToolResultBlock{ToolUseID: "call-1", Content: strings.Repeat("r", 120)},
		},
	}

	if got := estimateTokens([]types.Message{message}); got < 150 {
		t.Fatalf("estimateTokens() = %d, want all text, thinking, tool-use, and tool-result content counted", got)
	}
}

func TestOversizedToolResultKeepsHeadTailAndDurableReceipt(t *testing.T) {
	tools.ResetBudgetForNewTurn()
	t.Cleanup(tools.ResetBudgetForNewTurn)
	t.Setenv("HOME", t.TempDir())

	fullResult := "HEAD-" + strings.Repeat("x", 60_000) + "-TAIL"
	largeTool := tools.NewTool(tools.Tool{
		Name:               "large_output",
		MaxResultSizeChars: 1_024,
		Call: func(map[string]any, tools.Context, tools.CanUseToolFn, tools.OnProgress) (tools.ToolResult, error) {
			return tools.ToolResult{Data: fullResult}, nil
		},
		MapResult: func(result any, toolUseID string) types.ToolResultBlock {
			return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
		},
	})

	message, err := runSingleTool(
		tools.Context{AbortController: context.Background()},
		types.ToolUseBlock{ID: "large-1", Name: largeTool.Name, Input: map[string]any{}},
		types.Message{UUID: "assistant-1"},
		[]tools.Tool{largeTool},
		func(string, map[string]any, tools.Context) (tools.PermissionDecision, error) {
			return tools.PermissionDecision{Behavior: tools.Allow}, nil
		},
		nil,
	)
	if err != nil {
		t.Fatalf("runSingleTool() error = %v", err)
	}

	result, ok := message.Content[0].(types.ToolResultBlock)
	if !ok {
		t.Fatalf("result block type = %T, want types.ToolResultBlock", message.Content[0])
	}
	if !strings.Contains(result.Content, "HEAD-") || !strings.Contains(result.Content, "-TAIL") {
		t.Fatal("oversized result receipt must retain explicit head and tail content")
	}

	const marker = "[Full result stored at "
	start := strings.Index(result.Content, marker)
	if start < 0 {
		t.Fatalf("oversized result missing durable retrieval receipt:\n%s", result.Content)
	}
	start += len(marker)
	end := strings.Index(result.Content[start:], "]")
	if end < 0 {
		t.Fatalf("oversized result has malformed durable retrieval receipt:\n%s", result.Content)
	}
	storedPath := result.Content[start : start+end]
	if !filepath.IsAbs(storedPath) {
		t.Fatalf("stored result path = %q, want absolute path", storedPath)
	}
	stored, err := os.ReadFile(storedPath)
	if err != nil {
		t.Fatalf("read stored full result: %v", err)
	}
	if string(stored) != fullResult {
		t.Fatalf("stored full result length = %d, want %d", len(stored), len(fullResult))
	}
}

func firstText(messages []types.Message) string {
	for _, message := range messages {
		for _, block := range message.Content {
			if text, ok := block.(types.TextBlock); ok {
				return text.Text
			}
		}
	}
	return ""
}

func messagesContainText(messages []types.Message, want string) bool {
	for _, message := range messages {
		for _, block := range message.Content {
			if text, ok := block.(types.TextBlock); ok && strings.Contains(text.Text, want) {
				return true
			}
		}
	}
	return false
}
