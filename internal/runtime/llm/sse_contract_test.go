package llm

import (
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func TestReadSSEReconstructsFragmentedIndexedToolCalls(t *testing.T) {
	events := collectSSEEvents(t, context.Background(), fixtureBody(t, "fragmented-multi-tool.sse", 7))
	got := observeSSEEvents(events)

	if len(got.errors) != 0 {
		t.Fatalf("unexpected parser errors: %v", got.errors)
	}
	if want := []string{"chatcmpl-contract"}; !reflect.DeepEqual(got.starts, want) {
		t.Errorf("message starts = %#v, want %#v", got.starts, want)
	}
	if want := []string{"I will inspect ", "then update."}; !reflect.DeepEqual(got.text, want) {
		t.Errorf("text deltas = %#v, want %#v", got.text, want)
	}
	if want := []types.LLMToolUseDelta{
		{ID: "call_read", Name: "file_read", Delta: `{"path":"alpha.txt"}`},
		{ID: "call_write", Name: "write", Delta: `{"path":"beta.txt","content":"ok"}`},
	}; !reflect.DeepEqual(got.tools, want) {
		t.Errorf("tool deltas = %#v, want %#v", got.tools, want)
	}
	if want := []types.LLMMessageStop{{
		StopReason: "tool_calls",
		Model:      "fixture-model",
		Usage:      types.TokenUsage{InputTokens: 41, OutputTokens: 13},
	}}; !reflect.DeepEqual(got.stops, want) {
		t.Errorf("message stops = %#v, want %#v", got.stops, want)
	}
}

func TestReadSSEPreservesTextFinishMetadata(t *testing.T) {
	events := collectSSEEvents(t, context.Background(), fixtureBody(t, "text-finish.sse", 3))
	got := observeSSEEvents(events)

	if len(got.errors) != 0 {
		t.Fatalf("unexpected parser errors: %v", got.errors)
	}
	if want := []string{"done"}; !reflect.DeepEqual(got.text, want) {
		t.Errorf("text deltas = %#v, want %#v", got.text, want)
	}
	if want := []types.LLMMessageStop{{
		StopReason: "stop",
		Model:      "fixture-model",
		Usage:      types.TokenUsage{InputTokens: 3, OutputTokens: 1},
	}}; !reflect.DeepEqual(got.stops, want) {
		t.Errorf("message stops = %#v, want %#v", got.stops, want)
	}
}

func TestReadSSERejectsMalformedCompletedToolArguments(t *testing.T) {
	events := collectSSEEvents(t, context.Background(), fixtureBody(t, "malformed-tool-arguments.sse", 11))
	got := observeSSEEvents(events)

	if len(got.errors) != 1 {
		t.Fatalf("parser errors = %v, want one actionable error", got.errors)
	}
	assertErrorContains(t, got.errors[0], "tool call", "index 0", "call_bad", "invalid arguments JSON")
	if len(got.tools) != 0 {
		t.Errorf("malformed tool calls must not be emitted: %#v", got.tools)
	}
	if len(got.stops) != 0 {
		t.Errorf("malformed streams must not emit a successful stop: %#v", got.stops)
	}
}

func TestReadSSERejectsMalformedSSEJSON(t *testing.T) {
	events := collectSSEEvents(t, context.Background(), fixtureBody(t, "malformed-sse-json.sse", 5))
	got := observeSSEEvents(events)

	if len(got.errors) != 1 {
		t.Fatalf("parser errors = %v, want one actionable error", got.errors)
	}
	assertErrorContains(t, got.errors[0], "invalid SSE JSON")
	if len(got.stops) != 0 {
		t.Errorf("malformed streams must not emit a successful stop: %#v", got.stops)
	}
}

func TestReadSSECancellationInterruptsBlockedRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	body := newBlockingReadCloser()
	out := make(chan types.LLMEvent, 8)
	done := make(chan struct{})

	go func() {
		defer close(done)
		(&HTTPClient{}).readSSE(ctx, body, out)
	}()

	select {
	case <-body.started:
	case <-time.After(time.Second):
		t.Fatal("readSSE did not start reading")
	}
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		_ = body.Close()
		<-done
		t.Fatal("readSSE did not stop after context cancellation interrupted a blocked read")
	}

	got := observeSSEEvents(drainSSEEvents(out))
	if len(got.errors) != 1 {
		t.Fatalf("parser errors = %v, want context cancellation", got.errors)
	}
	if !errors.Is(got.errors[0], context.Canceled) {
		t.Errorf("parser error = %v, want context.Canceled", got.errors[0])
	}
	if !body.isClosed() {
		t.Error("response body was not closed after cancellation")
	}
}

type sseObservation struct {
	starts []string
	text   []string
	tools  []types.LLMToolUseDelta
	stops  []types.LLMMessageStop
	errors []error
}

func observeSSEEvents(events []types.LLMEvent) sseObservation {
	var got sseObservation
	for _, event := range events {
		switch event := event.(type) {
		case types.LLMMessageStart:
			got.starts = append(got.starts, event.ID)
		case types.LLMTextDelta:
			got.text = append(got.text, event.Delta)
		case types.LLMToolUseDelta:
			got.tools = append(got.tools, event)
		case types.LLMMessageStop:
			got.stops = append(got.stops, event)
		case types.LLMError:
			got.errors = append(got.errors, event.Error)
		}
	}
	return got
}

func collectSSEEvents(t *testing.T, ctx context.Context, body io.ReadCloser) []types.LLMEvent {
	t.Helper()

	out := make(chan types.LLMEvent, 32)
	(&HTTPClient{}).readSSE(ctx, body, out)
	return drainSSEEvents(out)
}

func drainSSEEvents(events <-chan types.LLMEvent) []types.LLMEvent {
	var collected []types.LLMEvent
	for event := range events {
		collected = append(collected, event)
	}
	return collected
}

func fixtureBody(t *testing.T, name string, maxReadSize int) io.ReadCloser {
	t.Helper()

	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %q: %v", name, err)
	}
	return &fragmentedReadCloser{
		reader:      strings.NewReader(string(data)),
		maxReadSize: maxReadSize,
	}
}

type fragmentedReadCloser struct {
	reader      io.Reader
	maxReadSize int
}

func (r *fragmentedReadCloser) Read(p []byte) (int, error) {
	if len(p) > r.maxReadSize {
		p = p[:r.maxReadSize]
	}
	return r.reader.Read(p)
}

func (r *fragmentedReadCloser) Close() error {
	return nil
}

type blockingReadCloser struct {
	started   chan struct{}
	unblock   chan struct{}
	startOnce sync.Once
	closeOnce sync.Once
}

func newBlockingReadCloser() *blockingReadCloser {
	return &blockingReadCloser{
		started: make(chan struct{}),
		unblock: make(chan struct{}),
	}
}

func (r *blockingReadCloser) Read([]byte) (int, error) {
	r.startOnce.Do(func() { close(r.started) })
	<-r.unblock
	return 0, io.ErrClosedPipe
}

func (r *blockingReadCloser) Close() error {
	r.closeOnce.Do(func() { close(r.unblock) })
	return nil
}

func (r *blockingReadCloser) isClosed() bool {
	select {
	case <-r.unblock:
		return true
	default:
		return false
	}
}

func assertErrorContains(t *testing.T, err error, fragments ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("error is nil")
	}
	for _, fragment := range fragments {
		if !strings.Contains(err.Error(), fragment) {
			t.Errorf("error %q does not contain %q", err, fragment)
		}
	}
}

// TestReadSSEIgnoresNvidiaReasoningContent pins the NVIDIA thinking
// stream contract: nemotron models interleave reasoning_content deltas
// with content deltas, and the parser must ignore the reasoning field
// (it carries no usable tool/text data for this harness) while keeping
// the visible content and finish metadata intact.
func TestReadSSEIgnoresNvidiaReasoningContent(t *testing.T) {
	events := collectSSEEvents(t, context.Background(), fixtureBody(t, "nvidia-thinking.sse", 3))
	got := observeSSEEvents(events)

	if len(got.errors) != 0 {
		t.Fatalf("unexpected parser errors: %v", got.errors)
	}
	if want := []string{"The answer"}; !reflect.DeepEqual(got.text, want) {
		t.Errorf("text deltas = %#v, want %#v (reasoning_content must be ignored)", got.text, want)
	}
	if len(got.stops) != 1 {
		t.Fatalf("message stops = %#v, want exactly one stop", got.stops)
	}
	if got.stops[0].StopReason != "stop" || got.stops[0].Model != "nvidia/nemotron-3.5-lightning-30b-a3b" {
		t.Errorf("stop = %#v, want stop reason 'stop' with the nvidia model id", got.stops[0])
	}
	wantUsage := types.TokenUsage{InputTokens: 5, OutputTokens: 2}
	if got.stops[0].Usage != wantUsage {
		t.Errorf("usage = %#v, want %#v", got.stops[0].Usage, wantUsage)
	}
}
