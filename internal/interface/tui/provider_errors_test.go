package tui

import (
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

// TestProviderErrorFeedbackNamesTheFix pins the actionability bar: the
// guidance names the command that fixes it, not just the feeling.
func TestProviderErrorFeedbackNamesTheFix(t *testing.T) {
	for _, category := range []ProviderErrorCategory{
		ProviderErrTimeout, ProviderErrRateLimit, ProviderErrAuth, ProviderErrModelNotFound,
	} {
		feedback := ProviderErrorFeedback(category, "x", false)
		if !strings.Contains(feedback, "[>]") {
			t.Errorf("%v feedback has no actionable hint: %q", category, feedback)
		}
	}
}
