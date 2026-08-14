package llm

import (
	"context"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestStream401DoesNotLeakAPIKey pins the redaction of provider error
// bodies: OpenAI/OpenRouter echo the key prefix in 401 messages, and the
// error surfaces in the chat pane and session files.
func TestStream401DoesNotLeakAPIKey(t *testing.T) {
	key := "sk-" + "secret-key-" + "1234567890"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Incorrect API key provided: " + key + ". You can find your API key at..."))
	}))
	defer srv.Close()

	client := NewHTTPClientWithBaseURL("openai", key, srv.URL)
	_, err := client.Stream(context.Background(), Request{Model: "gpt-4o", Messages: []types.Message{}})
	if err == nil {
		t.Fatal("expected error from 401")
	}
	if strings.Contains(err.Error(), key) {
		t.Fatalf("API key leaked into error: %v", err)
	}
	if !strings.Contains(err.Error(), "LLM API error 401") {
		t.Fatalf("unexpected error shape: %v", err)
	}
}
