package agent

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// TestProviderOverflowIsRecognized replays each verified provider's
// recorded context-overflow response through the real HTTP client and
// requires the resulting error to trigger prompt-too-long recovery.
// Each server words the overflow differently and delivers it
// differently (Lemonade in-band on a 200 stream, hosted APIs as HTTP
// 400), so a wording change on either side would otherwise silently
// turn recovery into a dead turn.
func TestProviderOverflowIsRecognized(t *testing.T) {
	cases := []struct {
		provider string
		fixture  string
	}{
		{"lemonade", "overflow.http"},
	}
	for _, tc := range cases {
		t.Run(tc.provider, func(t *testing.T) {
			err := replayOverflow(t, tc.provider, tc.fixture)
			if err == nil {
				t.Fatal("overflow fixture produced no error")
			}
			if !isPromptTooLongError(err) {
				t.Fatalf("overflow error not recognized as prompt-too-long: %v", err)
			}
		})
	}
}

// replayOverflow serves a raw HTTP response fixture from the llm
// package's contract testdata and returns the error the client surfaces,
// either from Stream itself or as an in-stream LLMError.
func replayOverflow(t *testing.T, provider, name string) error {
	t.Helper()
	path := filepath.Join("..", "runtime", "llm", "testdata", "contracts", provider, name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(raw)), nil)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatalf("read body %s: %v", path, err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := llm.NewHTTPClientWithBaseURL(provider, "sk-contract", srv.URL+"/v1")
	events, err := client.Stream(ctx, llm.Request{Model: "contract-model", Messages: []types.Message{}})
	if err != nil {
		return err
	}
	var streamErr error
	for event := range events {
		if e, ok := event.(types.LLMError); ok && streamErr == nil {
			streamErr = e.Error
		}
	}
	return streamErr
}
