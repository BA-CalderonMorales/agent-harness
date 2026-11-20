package llm

import (
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// Lemonade fixtures were captured from a real local server (Lemonade
// 2026.42.1, qwen3.5-9b-FLM loaded at a 4096-token window) with
// scripts/verify/capture-lemonade-contracts.py.

func TestLemonadeStreamContracts(t *testing.T) {
	cases := []struct {
		fixture string
		want    streamExpectation
	}{
		{
			// SSE keep-alive comments (": ping") precede the first chunk,
			// and usage arrives on the finish chunk with extra KV fields.
			fixture: "stream-text.http",
			want: streamExpectation{
				text: "ready",
				stop: &types.LLMMessageStop{
					StopReason: "stop",
					Model:      "qwen3.5:9b",
					Usage:      types.TokenUsage{InputTokens: 13, OutputTokens: 2},
				},
			},
		},
		{
			// Lemonade sends the whole tool call in one chunk, then a
			// stray newline of content, then finish_reason tool_calls.
			fixture: "stream-tool-call.http",
			want: streamExpectation{
				text: "\n",
				tools: []types.LLMToolUseDelta{{
					ID:    "call_1795187306",
					Name:  "get_weather",
					Delta: `{"city":"Paris"}`,
				}},
				stop: &types.LLMMessageStop{
					StopReason: "tool_calls",
					Model:      "qwen3.5:9b",
					Usage:      types.TokenUsage{InputTokens: 282, OutputTokens: 26},
				},
			},
		},
		{
			// An overflow is NOT an HTTP 400: Lemonade answers 200 with an
			// SSE error object and never sends [DONE]. The sentence must
			// survive so the agent's prompt-too-long recovery sees it.
			fixture: "overflow.http",
			want: streamExpectation{
				errContains: []string{"LLM API error 400", "Max length reached!"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			got, srv, err := streamContract(t, "lemonade", tc.fixture)
			assertStreamContract(t, got, err, tc.want)

			requests := srv.seen()
			if len(requests) != 1 {
				t.Fatalf("local providers must not retry: saw %d requests", len(requests))
			}
			if requests[0].Method != "POST" || requests[0].Path != "/v1/chat/completions" {
				t.Errorf("request = %s %s, want POST /v1/chat/completions", requests[0].Method, requests[0].Path)
			}
			if auth := requests[0].Header.Get("Authorization"); auth != "" {
				t.Errorf("local Lemonade request carried Authorization %q", auth)
			}
		})
	}
}

// TestLemonadeModelsContract pins that the app reads context_length (the
// window the server actually loaded) and not max_context_window (what
// the model could support). The captured server loaded qwen3.5-9b-FLM
// at 4096 while advertising a 262144 maximum; budgeting against the
// maximum would let the context bar run far past the real window.
func TestLemonadeModelsContract(t *testing.T) {
	infos, srv := listModelsContract(t, "lemonade", "models.http")

	want := map[string]int{
		"qwen3.5-2b-FLM": 262144,
		"qwen3.5-4b-FLM": 262144,
		"qwen3.5-9b-FLM": 4096,
	}
	got := contextLengths(infos)
	if len(got) != len(want) {
		t.Fatalf("models = %v, want %v", got, want)
	}
	for id, ctx := range want {
		if got[id] != ctx {
			t.Errorf("%s context = %d, want %d (context_length, not max_context_window)", id, got[id], ctx)
		}
	}

	requests := srv.seen()
	if len(requests) != 1 || requests[0].Path != "/v1/models" {
		t.Fatalf("requests = %#v, want one GET /v1/models", requests)
	}
	if auth := requests[0].Header.Get("Authorization"); auth != "" {
		t.Errorf("local Lemonade models request carried Authorization %q", auth)
	}
}
