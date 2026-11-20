package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// lemonadeOverflow is the error HTTPClient returns when Lemonade refuses a
// request that is larger than the window it loaded the model with.
const lemonadeOverflow = "LLM API error 400: Max length reached!"

// windowedClient behaves like a local server with a fixed context window.
// It refuses any request whose input exceeds the window the way HTTPClient
// surfaces a non-200 response: an error from Stream itself, not a stream
// event. Summary requests answer with a numbered part that carries the goal.
type windowedClient struct {
	mu        sync.Mutex
	window    int
	requests  []llm.Request
	refused   int
	summaries int
}

func windowedRequestTokens(req llm.Request) int {
	return (len(req.SystemPrompt)+3)/4 + estimateTokens(req.Messages)
}

func (c *windowedClient) Stream(_ context.Context, req llm.Request) (<-chan types.LLMEvent, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requests = append(c.requests, req)
	if windowedRequestTokens(req) > c.window {
		c.refused++
		return nil, errors.New(lemonadeOverflow)
	}
	text := "done"
	if strings.Contains(req.SystemPrompt, "context summarizer") {
		c.summaries++
		text = fmt.Sprintf("ORIGINAL-GOAL carried forward; summary part %d", c.summaries)
	}
	events := compactionTextEvents(text)
	out := make(chan types.LLMEvent, len(events))
	for _, event := range events {
		out <- event
	}
	close(out)
	return out, nil
}

func (c *windowedClient) snapshot() (requests []llm.Request, refused int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]llm.Request(nil), c.requests...), c.refused
}

func splitWindowedRequests(requests []llm.Request) (summaries, main []llm.Request) {
	for _, request := range requests {
		if strings.Contains(request.SystemPrompt, "context summarizer") {
			summaries = append(summaries, request)
		} else {
			main = append(main, request)
		}
	}
	return summaries, main
}

// windowHistory is a conversation of roughly 100 estimated tokens per
// message, opening with the goal compaction must carry forward.
func windowHistory(count int) []types.Message {
	messages := make([]types.Message, 0, count)
	for i := 0; i < count; i++ {
		role := types.RoleUser
		if i%2 == 1 {
			role = types.RoleAssistant
		}
		text := fmt.Sprintf("turn-%03d: %s", i, strings.Repeat("context ", 50))
		if i == 0 {
			text = "ORIGINAL-GOAL: keep the local model usable; " + text
		}
		messages = append(messages, types.Message{
			UUID:    fmt.Sprintf("window-%03d", i),
			Role:    role,
			Content: []types.ContentBlock{types.TextBlock{Text: text}},
		})
	}
	return messages
}

func runWindowedQuery(t *testing.T, loop *Loop, history []types.Message) Terminal {
	t.Helper()
	params := edgeParams(nil)
	params.Messages = history
	state := edgeLoopState(history)
	out := make(chan types.StreamEvent, 512)
	return loop.queryLoop(context.Background(), params, state, out)
}

// The history is about three times the window, so the prefix compaction
// summarizes cannot go to the model in one request.
func TestAutoCompactionFitsEverySummaryRequestInASmallWindow(t *testing.T) {
	client := &windowedClient{window: 4096}
	loop := NewLoop(client)
	loop.Config.BlockingTokenLimit = 4096
	loop.Config.DefaultMaxTurns = 1

	terminal := runWindowedQuery(t, loop, windowHistory(120))
	if terminal.Reason != TerminalReasonComplete {
		t.Fatalf("terminal reason = %q, want %q (error: %v)", terminal.Reason, TerminalReasonComplete, terminal.Error)
	}

	requests, refused := client.snapshot()
	if refused != 0 {
		t.Fatalf("the server refused %d request(s); every request must fit the %d-token window", refused, client.window)
	}
	summaries, main := splitWindowedRequests(requests)
	if len(summaries) < 2 {
		t.Fatalf("summary requests = %d, want the oversized prefix split across several", len(summaries))
	}
	if !strings.Contains(firstText(summaries[1].Messages), "summary part 1") {
		t.Fatal("the second summary request did not carry the first part's summary forward")
	}
	if !strings.Contains(firstText(summaries[0].Messages), "ORIGINAL-GOAL") {
		t.Fatal("the first summary request did not receive the start of the conversation")
	}
	if len(main) != 1 || !messagesContainText(main[0].Messages, "ORIGINAL-GOAL carried forward") {
		t.Fatalf("the main request did not use the compacted history (main requests: %d)", len(main))
	}
}

// The loop believes the window is 262144, as it did when Lemonade
// advertised the model's maximum while serving it with 4096. The refusal
// must trigger compaction that shrinks to what the server accepts.
func TestProviderOverflowShrinksAnOverstatedWindowAndRecovers(t *testing.T) {
	client := &windowedClient{window: 4096}
	loop := NewLoop(client)
	loop.Config.BlockingTokenLimit = 262144
	loop.Config.DefaultMaxTurns = 1

	terminal := runWindowedQuery(t, loop, windowHistory(120))
	if terminal.Reason != TerminalReasonComplete {
		t.Fatalf("terminal reason = %q, want %q (error: %v)", terminal.Reason, TerminalReasonComplete, terminal.Error)
	}

	requests, _ := client.snapshot()
	_, main := splitWindowedRequests(requests)
	if len(main) < 2 {
		t.Fatalf("main requests = %d, want the refused request plus a retry", len(main))
	}
	last := main[len(main)-1]
	if windowedRequestTokens(last) > client.window {
		t.Fatalf("the retry used %d tokens, more than the %d-token window", windowedRequestTokens(last), client.window)
	}
	if !messagesContainText(last.Messages, "ORIGINAL-GOAL carried forward") {
		t.Fatal("the retry lost the original goal")
	}
}

func TestPromptTooLongErrorRecognizesProviderOverflowWording(t *testing.T) {
	overflow := []string{
		"prompt_too_long: context length exceeded",
		lemonadeOverflow,
		"LLM API error 400: This model's maximum context length is 8192 tokens",
		"LLM API error 400: the request exceeds the available context size, try increasing it",
		"LLM API error 400: context_length_exceeded",
		"LLM API error 400: Input is too long for requested model.",
		"LLM API error 413: prompt is too long",
	}
	for _, message := range overflow {
		if !isPromptTooLongError(errors.New(message)) {
			t.Errorf("not recognized as prompt too long: %q", message)
		}
	}
	unrelated := []string{
		"LLM API error 401: invalid api key",
		"LLM API error 429: Rate limit exceeded",
		"LLM API error 400: max_tokens must be at least 1",
		"Provider stream error: connection reset by peer",
	}
	for _, message := range unrelated {
		if isPromptTooLongError(errors.New(message)) {
			t.Errorf("wrongly recognized as prompt too long: %q", message)
		}
	}
}
