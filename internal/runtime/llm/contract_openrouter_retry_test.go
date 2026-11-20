package llm

import "testing"

// TestOpenRouter429RetryAfterContract pins how Retry-After steers the
// hosted retry: a short window is honored (the client re-sends, up to
// three attempts) while a window past maxRetryAfter is not waited out
// and surfaces on the first response. In both cases the surfaced error
// keeps the header, so the user sees when to try again.
func TestOpenRouter429RetryAfterContract(t *testing.T) {
	cases := []struct {
		fixture      string
		wantRequests int
		wantRetry    string
	}{
		{"error-429-retry-after-0.http", 3, "retry_after=0"},
		{"error-429-retry-after-60.http", 1, "retry_after=60"},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			got, srv, err := streamContract(t, "openrouter", tc.fixture)
			assertStreamContract(t, got, err, streamExpectation{errContains: []string{
				"LLM API error 429", "Provider returned error", "type=rate_limit_exceeded",
				"provider=upstream-a", "limit=50", "remaining=0", tc.wantRetry,
			}})
			if n := len(srv.seen()); n != tc.wantRequests {
				t.Errorf("requests = %d, want %d (Retry-After steers the retry)", n, tc.wantRequests)
			}
		})
	}
}
