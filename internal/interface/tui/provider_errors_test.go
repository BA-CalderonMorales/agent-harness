package tui

import (
	"errors"
	"strings"
	"testing"
)

// TestClassifyProviderError pins the taxonomy: every provider dialect
// for the same failure maps to one category.
func TestClassifyProviderError(t *testing.T) {
	cases := []struct {
		err  string
		want ProviderErrorCategory
	}{
		{"Get \"http://127.0.0.1:8080/v1/models\": dial tcp 127.0.0.1:8080: connect: connection refused", ProviderErrConnection},
		{"context deadline exceeded after 30s", ProviderErrTimeout},
		{"request timed out", ProviderErrTimeout},
		{"429 too many requests", ProviderErrRateLimit},
		{"rate limit exceeded for org", ProviderErrRateLimit},
		{"LLM API error 402: Insufficient credits", ProviderErrCredits},
		{"LLM API error 402: This request would exceed your available credits", ProviderErrCredits},
		{"LLM API error 429: Rate limit exceeded (type=rate_limit_exceeded; provider=some-upstream)", ProviderErrRateLimit},
		{"401 unauthorized: invalid api key", ProviderErrAuth},
		{"model gpt-x does not exist", ProviderErrModelNotFound},
		{"404 not found", ProviderErrModelNotFound},
		{"something entirely novel", ProviderErrUnknown},
		{"tool repetition limit: ls_recursive requested the same input 3 times", ProviderErrToolLoop},
		{"agent turn ended: blocking_limit", ProviderErrAgentLimit},
		{"agent turn ended: max_turns", ProviderErrAgentLimit},
	}
	for _, tc := range cases {
		if got := ClassifyProviderError(tc.err); got != tc.want {
			t.Errorf("ClassifyProviderError(%q) = %v, want %v", tc.err, got, tc.want)
		}
	}
}

func TestAgentLoopFailureDoesNotSuggestChangingProviders(t *testing.T) {
	for _, text := range []string{
		"tool repetition limit: ls_recursive requested the same input 3 times; the repeated call was not executed",
		"agent turn ended: blocking_limit",
	} {
		feedback := ProviderErrorFeedback(ClassifyProviderError(text), text, true)
		if strings.Contains(feedback, "/model") || strings.Contains(feedback, "Settings") || strings.Contains(feedback, "isn't responding") {
			t.Fatalf("agent limit incorrectly blamed the provider: %s", feedback)
		}
		if !strings.Contains(feedback, "continue") {
			t.Fatalf("missing continuation guidance: %s", feedback)
		}
	}
}

// TestProviderErrorFeedbackLocalHosted pins the context split: the same
// connection failure points local users at their server and hosted
// users at their key.
func TestProviderErrorFeedbackLocalHosted(t *testing.T) {
	local := ProviderErrorFeedback(ProviderErrConnection, "dial tcp", true)
	hosted := ProviderErrorFeedback(ProviderErrConnection, "dial tcp", false)
	if !strings.Contains(local, "local model server") {
		t.Errorf("local feedback lost the server hint: %q", local)
	}
	if !strings.Contains(hosted, "API key") {
		t.Errorf("hosted feedback lost the key hint: %q", hosted)
	}
	if local == hosted {
		t.Error("local and hosted feedback must differ")
	}
}

// TestRateLimitFeedbackKeepsTheProviderDetail pins the diagnosis
// contract from the OpenRouter report: the pane must show whether the
// 429 came from the platform free-tier cap, an upstream provider, or a
// credit/budget rejection. The old feedback replaced all of that with
// one generic sentence.
func TestRateLimitFeedbackKeepsTheProviderDetail(t *testing.T) {
	detail := "LLM API error 429: Rate limit exceeded (type=rate_limit_exceeded; provider=some-upstream; remaining=0; reset=42)"
	feedback := ProviderErrorFeedback(ProviderErrRateLimit, detail, false)
	if !strings.Contains(feedback, "some-upstream") || !strings.Contains(feedback, "remaining=0") {
		t.Fatalf("rate-limit feedback dropped the provider detail: %q", feedback)
	}
	if !strings.Contains(feedback, "upstream provider") {
		t.Fatalf("rate-limit feedback does not explain upstream capacity: %q", feedback)
	}

	credits := ProviderErrorFeedback(ProviderErrCredits, "LLM API error 402: Insufficient credits", false)
	if !strings.Contains(credits, "not a request-rate limit") {
		t.Fatalf("credits feedback mislabels a 402 as a rate limit: %q", credits)
	}
	if !strings.Contains(credits, "Insufficient credits") {
		t.Fatalf("credits feedback dropped the provider sentence: %q", credits)
	}
}

// TestOpenRouterRateLimitPaneShowsTheCause is the user-facing check for
// the OpenRouter report: the transcript must carry the provider's own
// limit detail and the free-tier numbers, not just "rate limit".
func TestOpenRouterRateLimitPaneShowsTheCause(t *testing.T) {
	m := NewChatModel()
	m.provider = "openrouter"
	serviceErr := errors.New("LLM API error 429: Rate limit exceeded (type=rate_limit_exceeded; remaining=0; reset=42)")

	model, _ := m.Update(AgentErrorMsg{Error: serviceErr})
	m = model.(ChatModel)
	if len(m.messages) == 0 {
		t.Fatal("provider error added no transcript message")
	}
	pane := m.messages[len(m.messages)-1].Content
	for _, want := range []string{"remaining=0", "reset=42", "20 requests/min", "50/day", "openrouter.ai/settings/credits"} {
		if !strings.Contains(pane, want) {
			t.Fatalf("rate-limit pane = %q, missing %q", pane, want)
		}
	}
}

// TestCreditsFailureIsNotLabeledRateLimit keeps a 402 from sending the
// user hunting for a request-rate window that does not exist.
func TestCreditsFailureIsNotLabeledRateLimit(t *testing.T) {
	m := NewChatModel()
	m.provider = "openrouter"
	serviceErr := errors.New("LLM API error 402: Insufficient credits")

	model, _ := m.Update(AgentErrorMsg{Error: serviceErr})
	m = model.(ChatModel)
	pane := m.messages[len(m.messages)-1].Content
	if strings.Contains(pane, "Rate limit or quota exceeded") {
		t.Fatalf("402 was labeled a rate limit: %q", pane)
	}
	if !strings.Contains(pane, "credit or budget rejection") {
		t.Fatalf("402 pane does not name the credits cause: %q", pane)
	}
}

// TestProviderErrorFeedbackNamesTheFix pins the actionability bar: the
// guidance names the command that fixes it, not just the feeling.
func TestProviderErrorFeedbackNamesTheFix(t *testing.T) {
	for _, category := range []ProviderErrorCategory{
		ProviderErrTimeout, ProviderErrRateLimit, ProviderErrCredits, ProviderErrAuth, ProviderErrModelNotFound,
	} {
		feedback := ProviderErrorFeedback(category, "x", false)
		if !strings.Contains(feedback, "[>]") {
			t.Errorf("%v feedback has no actionable hint: %q", category, feedback)
		}
	}
}
