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

// TestAPIErrorCarriesRateLimitMetadata pins the diagnosis contract:
// OpenRouter's 429 is the same sentence for a platform cap, an upstream
// provider at capacity, and a budget rejection. The error must carry
// the metadata and X-RateLimit headers that tell them apart.
func TestAPIErrorCarriesRateLimitMetadata(t *testing.T) {
	err := &APIError{
		StatusCode: 429,
		Message:    "Rate limit exceeded",
		Metadata: APIErrorMetadata{
			ErrorType:    "rate_limit_exceeded",
			ProviderCode: "some-upstream",
			RemedyHint:   "Retry after the reset window.",
		},
		Limit:      "20",
		Remaining:  "0",
		Reset:      "42",
		RetryAfter: "3",
	}
	got := err.Error()
	for _, want := range []string{
		"LLM API error 429", "Rate limit exceeded", "type=rate_limit_exceeded",
		"provider=some-upstream", "limit=20", "remaining=0", "reset=42",
		"retry_after=3", "Retry after the reset window.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("APIError.Error() = %q, missing %q", got, want)
		}
	}
}

func TestParseStreamErrorObject(t *testing.T) {
	err := parseStreamError([]byte(`{"code":429,"message":"Rate limit exceeded","metadata":{"error_type":"rate_limit_exceeded","provider_code":"p1"}}`))
	if err == nil {
		t.Fatal("parseStreamError returned nil")
	}
	if err.StatusCode != 429 || err.Message != "Rate limit exceeded" {
		t.Fatalf("parsed error = %+v", err)
	}
	if err.Metadata.ErrorType != "rate_limit_exceeded" || err.Metadata.ProviderCode != "p1" {
		t.Fatalf("metadata = %+v", err.Metadata)
	}
}

// TestStreamErrorIncludesRateLimitHeaders proves the HTTP path keeps the
// headers OpenRouter sends on a platform limit. Retry-After is long
// enough that the bounded retry declines to wait, so this is the first
// and only response.
func TestStreamErrorIncludesRateLimitHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Limit", "20")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", "42")
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":429,"message":"Rate limit exceeded","metadata":{"error_type":"rate_limit_exceeded"}}}`))
	}))
	defer srv.Close()

	client := NewHTTPClientWithBaseURL("openrouter", "test-key", srv.URL)
	_, err := client.Stream(context.Background(), Request{Model: "m"})
	if err == nil {
		t.Fatal("expected a 429 error")
	}
	msg := err.Error()
	for _, want := range []string{"429", "Rate limit exceeded", "type=rate_limit_exceeded", "remaining=0", "reset=42", "retry_after=60"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error = %q, missing %q", msg, want)
		}
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
