package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// writeSSE completes a one-text-chunk stream so Stream can return a
// successful channel after a retried status.
func writeSSE(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	_, _ = w.Write([]byte(
		"data: {\"id\":\"chatcmpl-retry\",\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n" +
			"data: {\"id\":\"chatcmpl-retry\",\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n" +
			"data: [DONE]\n"))
}

func collectText(events <-chan types.LLMEvent) (string, error) {
	var text strings.Builder
	for event := range events {
		switch e := event.(type) {
		case types.LLMTextDelta:
			text.WriteString(e.Delta)
		case types.LLMError:
			return text.String(), e.Error
		}
	}
	return text.String(), nil
}

// TestStreamRetriesTransientHostedStatus pins the free-tier solidity fix:
// OpenRouter's shared pool answers 429 (or a gateway 502/503/504) before
// any token, and one bounded retry turns a dead turn into a normal one.
func TestStreamRetriesTransientHostedStatus(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"free pool busy"}}`))
			return
		}
		writeSSE(w)
	}))
	defer srv.Close()

	client := NewHTTPClientWithBaseURL("openrouter", "test-key", srv.URL)
	events, err := client.Stream(context.Background(), Request{Model: "nvidia/nemotron-3.5-lightning:free"})
	if err != nil {
		t.Fatalf("Stream() error = %v, want retry to succeed", err)
	}
	text, streamErr := collectText(events)
	if streamErr != nil {
		t.Fatalf("stream error = %v", streamErr)
	}
	if text != "ok" {
		t.Fatalf("streamed text = %q, want ok", text)
	}
	if got := attempts.Load(); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
}

// TestStreamDoesNotRetryRateLimitWithoutWindow: a free-tier cap does not
// clear in 400ms, and each retry spends another request against the same
// cap. No Retry-After means show the provider's reason immediately.
func TestStreamDoesNotRetryRateLimitWithoutWindow(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":429,"message":"Rate limit exceeded"}}`))
	}))
	defer srv.Close()

	client := NewHTTPClientWithBaseURL("openrouter", "test-key", srv.URL)
	_, err := client.Stream(context.Background(), Request{Model: "m"})
	if err == nil || !strings.Contains(err.Error(), "Rate limit exceeded") {
		t.Fatalf("error = %v, want the provider sentence", err)
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("attempts = %d, want 1 (no blind 429 retry)", got)
	}
}

// TestStreamDoesNotRetryLocalProvider keeps the local failure mode fast:
// a dead llama.cpp/ollama server should point at the server immediately,
// not spend seconds retrying a port that is not listening.
func TestStreamDoesNotRetryLocalProvider(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"local busy"}}`))
	}))
	defer srv.Close()

	client := NewHTTPClientWithBaseURL("local", "", srv.URL)
	_, err := client.Stream(context.Background(), Request{Model: "local-model"})
	if err == nil {
		t.Fatal("expected the local 429 to surface")
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("local attempts = %d, want 1", got)
	}
}

// TestStreamRetryStopsAtCap: a persistently overloaded provider returns
// its own error after bounded attempts instead of looping forever.
func TestStreamRetryStopsAtCap(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"message":"provider overloaded"}}`))
	}))
	defer srv.Close()

	client := NewHTTPClientWithBaseURL("openrouter", "test-key", srv.URL)
	_, err := client.Stream(context.Background(), Request{Model: "m"})
	if err == nil || !strings.Contains(err.Error(), "provider overloaded") {
		t.Fatalf("error = %v, want the provider sentence", err)
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("attempts = %d, want 3", got)
	}
}

// TestStreamDeclinesLongRetryAfter keeps a 60-second Retry-After from
// turning the turn into a silent stall: the rate-limit error lands now.
func TestStreamDeclinesLongRetryAfter(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"slow down"}}`))
	}))
	defer srv.Close()

	client := NewHTTPClientWithBaseURL("openrouter", "test-key", srv.URL)
	started := time.Now()
	_, err := client.Stream(context.Background(), Request{Model: "m"})
	if err == nil || !strings.Contains(err.Error(), "slow down") {
		t.Fatalf("error = %v, want the provider sentence", err)
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("attempts = %d, want 1 (no long wait)", got)
	}
	if elapsed := time.Since(started); elapsed > maxRetryAfter {
		t.Fatalf("Stream waited %s for a long Retry-After", elapsed)
	}
}
