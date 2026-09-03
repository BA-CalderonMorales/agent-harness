package llm

import (
	"testing"
	"time"
)

// TestNewHTTPClientWithBaseURLTimeout verifies the HTTP client timeout is
// configurable per the app's config (local providers need multi-minute
// windows for slow CPU prompt eval).
func TestNewHTTPClientWithBaseURLTimeout(t *testing.T) {
	client := NewHTTPClientWithBaseURLTimeout("local", "", "http://127.0.0.1:8080/v1", 45*time.Minute)
	if got := client.HTTPClient.Timeout; got != 45*time.Minute {
		t.Errorf("HTTPClient.Timeout = %v, want 45m", got)
	}
}

// TestNewHTTPClientKeepsLegacyTimeout verifies the plain constructor keeps
// the historic tight default for callers that do not opt in.
func TestNewHTTPClientKeepsLegacyTimeout(t *testing.T) {
	client := NewHTTPClientWithBaseURL("local", "", "http://127.0.0.1:8080/v1")
	if got := client.HTTPClient.Timeout; got != defaultHTTPTimeout {
		t.Errorf("HTTPClient.Timeout = %v, want legacy default %v", got, defaultHTTPTimeout)
	}
}
