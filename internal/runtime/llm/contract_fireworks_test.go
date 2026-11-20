package llm

import (
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// Fireworks fixtures are constructed (no key on the capture machine)
// from the shapes HTTPClient already parses; see
// testdata/contracts/README.md.

func TestFireworksStreamContracts(t *testing.T) {
	cases := []struct {
		fixture string
		want    streamExpectation
	}{
		{
			// reasoning_content dialect, and a "length" finish that the
			// agent's max-output recovery keys on.
			fixture: "stream-text.http",
			want: streamExpectation{
				text: "One two three",
				stop: &types.LLMMessageStop{
					StopReason: "length",
					Model:      "accounts/fireworks/models/glm-5p3-flash",
					Usage:      types.TokenUsage{InputTokens: 18, OutputTokens: 4},
				},
			},
		},
		{
			// Two parallel calls whose argument fragments interleave in
			// one chunk; each must accumulate under its own index.
			fixture: "stream-tool-call.http",
			want: streamExpectation{
				text: "Checking both.",
				tools: []types.LLMToolUseDelta{
					{ID: "call_fw_a", Name: "bash", Delta: `{"command":"ls"}`},
					{ID: "call_fw_b", Name: "file_read", Delta: `{"path":"go.mod"}`},
				},
				stop: &types.LLMMessageStop{
					StopReason: "tool_calls",
					Model:      "accounts/fireworks/models/glm-5p3-flash",
					Usage:      types.TokenUsage{InputTokens: 402, OutputTokens: 37, CacheReadInputTokens: 384},
				},
			},
		},
		{
			fixture: "error-401.http",
			want:    streamExpectation{errContains: []string{"LLM API error 401", "The API key you provided is invalid."}},
		},
		{
			fixture: "error-429.http",
			want:    streamExpectation{errContains: []string{"LLM API error 429", "Request rate limit exceeded"}},
		},
		{
			fixture: "overflow-400.http",
			want:    streamExpectation{errContains: []string{"LLM API error 400", "The prompt is too long"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			got, srv, err := streamContract(t, "fireworks", tc.fixture)
			assertStreamContract(t, got, err, tc.want)

			requests := srv.seen()
			if len(requests) != 1 {
				t.Fatalf("saw %d requests, want exactly one (no retry for these responses)", len(requests))
			}
			assertHostedHeaders(t, requests[0], "/v1/chat/completions")
			if got := requests[0].Header.Get("X-Title"); got != "" {
				t.Errorf("Fireworks request carried OpenRouter's X-Title %q", got)
			}
		})
	}
}

func TestFireworksModelsContract(t *testing.T) {
	infos, srv := listModelsContract(t, "fireworks", "models.http")
	got := contextLengths(infos)
	want := map[string]int{
		"accounts/fireworks/models/glm-5p3-flash":          1048576,
		"accounts/fireworks/models/deepseek-v4-flash-0731": 1048576,
	}
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
