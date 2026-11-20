package llm

import (
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// OpenRouter fixtures are constructed (no key on the capture machine)
// from the shapes HTTPClient already parses; see
// testdata/contracts/README.md.

func TestOpenRouterStreamContracts(t *testing.T) {
	cases := []struct {
		fixture string
		want    streamExpectation
	}{
		{
			// Reasoning before content, a ": OPENROUTER PROCESSING"
			// comment, and usage on a trailing chunk with empty choices.
			fixture: "stream-text.http",
			want: streamExpectation{
				text: "Hello there.",
				stop: &types.LLMMessageStop{
					StopReason: "stop",
					Model:      "openai/gpt-oss-120b:free",
					Usage:      types.TokenUsage{InputTokens: 24, OutputTokens: 9, CacheReadInputTokens: 16},
				},
			},
		},
		{
			// Arguments split mid-key and mid-value across chunks.
			fixture: "stream-tool-call.http",
			want: streamExpectation{
				tools: []types.LLMToolUseDelta{{ID: "call_or_1", Name: "file_read", Delta: `{"path":"README.md"}`}},
				stop: &types.LLMMessageStop{
					StopReason: "tool_calls",
					Model:      "z-ai/glm-4.6",
					Usage:      types.TokenUsage{InputTokens: 310, OutputTokens: 21},
				},
			},
		},
		{
			fixture: "mid-stream-rate-limit.http",
			want: streamExpectation{errContains: []string{
				"429", "Rate limit exceeded", "type=rate_limit_exceeded", "provider=upstream-a",
			}},
		},
		{
			fixture: "error-401.http",
			want:    streamExpectation{errContains: []string{"LLM API error 401", "No auth credentials found"}},
		},
		{
			fixture: "error-402.http",
			want: streamExpectation{errContains: []string{
				"LLM API error 402", "Insufficient credits", "type=insufficient_credits",
				"Add credits or pick a free model.",
			}},
		},
		{
			// No Retry-After: a daily cap must surface on the first
			// response, with the metadata and X-RateLimit-* headers.
			fixture: "error-429.http",
			want: streamExpectation{errContains: []string{
				"LLM API error 429", "free-models-per-day", "type=rate_limit_exceeded",
				"limit_source=platform", "limit=50", "remaining=0", "reset=1795190400000",
				"Free model daily limit reached.",
			}},
		},
		{
			fixture: "overflow-400.http",
			want:    streamExpectation{errContains: []string{"LLM API error 400", "maximum context length is 131072"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			got, srv, err := streamContract(t, "openrouter", tc.fixture)
			assertStreamContract(t, got, err, tc.want)

			requests := srv.seen()
			if len(requests) != 1 {
				t.Fatalf("saw %d requests, want exactly one (no retry for these responses)", len(requests))
			}
			assertHostedHeaders(t, requests[0], "/v1/chat/completions")
			if got := requests[0].Header.Get("X-Title"); got != "agent-harness" {
				t.Errorf("X-Title = %q, want agent-harness", got)
			}
			if got := requests[0].Header.Get("HTTP-Referer"); got == "" {
				t.Error("OpenRouter request is missing HTTP-Referer")
			}
		})
	}
}

func TestOpenRouterModelsContract(t *testing.T) {
	infos, srv := listModelsContract(t, "openrouter", "models.http")
	got := contextLengths(infos)
	want := map[string]int{"openai/gpt-oss-120b:free": 131072, "z-ai/glm-4.6": 202752}
	if len(got) != len(want) {
		t.Fatalf("models = %v, want %v", got, want)
	}
	for id, ctx := range want {
		if got[id] != ctx {
			t.Errorf("%s context = %d, want %d", id, got[id], ctx)
		}
	}
	assertHostedHeaders(t, srv.seen()[0], "/v1/models")
}

// assertHostedHeaders pins what every hosted request carries: the path
// and the bearer key.
func assertHostedHeaders(t *testing.T, req replayedRequest, path string) {
	t.Helper()
	if req.Path != path {
		t.Errorf("path = %q, want %q", req.Path, path)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer "+contractKey {
		t.Errorf("Authorization = %q, want the bearer key", got)
	}
}
