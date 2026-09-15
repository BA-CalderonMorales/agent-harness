package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// TestAPIErrorMessageExtractsEnvelope covers the provider dialects: the
// human sentence must come out, the JSON shell must not.
func TestAPIErrorMessageExtractsEnvelope(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"nested message", `{"error":{"message":"model not found","code":404}}`, "model not found"},
		{"anthropic shape", `{"type":"error","error":{"type":"invalid_request_error","message":"max_tokens too large"}}`, "max_tokens too large"},
		{"error as string", `{"error":"invalid api key"}`, "invalid api key"},
		{"top-level message", `{"message":"rate limit exceeded"}`, "rate limit exceeded"},
		{"detail", `{"detail":"unprocessable entity"}`, "unprocessable entity"},
		{"plain text", `Bad Gateway`, "Bad Gateway"},
		{"empty", ``, "no response body"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := apiErrorMessage([]byte(tc.body)); got != tc.want {
				t.Fatalf("apiErrorMessage(%q) = %q, want %q", tc.body, got, tc.want)
			}
		})
	}
}

// TestStreamErrorUnwrapsJSONEnvelope pins the chat-pane contract: a
// provider 400 reaches the user as a sentence, not a JSON blob.
func TestStreamErrorUnwrapsJSONEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"The model 'x/y' does not exist","code":400}}`))
	}))
	defer srv.Close()

	client := NewHTTPClientWithBaseURL("nvidia", "nvapi-test", srv.URL)
	_, err := client.Stream(context.Background(), Request{Model: "x/y", Messages: []types.Message{}})
	if err == nil {
		t.Fatal("expected error from 400")
	}
	msg := err.Error()
	if !strings.Contains(msg, "does not exist") {
		t.Fatalf("error lost the provider message: %v", err)
	}
	if strings.Contains(msg, `{"error"`) || strings.Contains(msg, "code\":400") {
		t.Fatalf("error leaked the JSON envelope: %v", err)
	}
}
