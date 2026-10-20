package builtin

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
)

func TestValidateFetchURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"https", "https://example.com/docs", false},
		{"http", "http://example.com", false},
		{"uppercase scheme", "HTTPS://example.com", false},
		{"file scheme", "file:///etc/passwd", true},
		{"ftp scheme", "ftp://example.com/x", true},
		{"javascript scheme", "javascript:alert(1)", true},
		{"no scheme", "example.com", true},
		{"scheme prefix only", "httpfoo://example.com", true},
		{"no host", "https://", true},
		{"embedded credentials", "https://user:pass@example.com", true},
		{"empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFetchURL(tt.url)
			if tt.wantErr && err == nil {
				t.Fatalf("validateFetchURL(%q) = nil, want an error", tt.url)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("validateFetchURL(%q) = %v, want nil", tt.url, err)
			}
		})
	}
}

func TestBlockedFetchIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1",
		"127.1.2.3",
		"::1",
		"0.0.0.0",
		"10.0.0.1",
		"172.16.0.1",
		"192.168.1.1",
		"169.254.169.254", // cloud metadata
		"100.64.0.1",      // carrier-grade NAT
		"224.0.0.1",       // multicast
		"fe80::1",
		"fd00::1",
		"::ffff:127.0.0.1",
	}
	for _, raw := range blocked {
		if !blockedFetchIP(net.ParseIP(raw)) {
			t.Errorf("blockedFetchIP(%s) = false, want true", raw)
		}
	}
	if !blockedFetchIP(nil) {
		t.Error("blockedFetchIP(nil) = false, want true")
	}

	allowed := []string{"1.1.1.1", "8.8.8.8", "93.184.216.34", "2606:4700:4700::1111"}
	for _, raw := range allowed {
		if blockedFetchIP(net.ParseIP(raw)) {
			t.Errorf("blockedFetchIP(%s) = true, want false", raw)
		}
	}
}

func TestWebFetchRejectsPrivateDestinationByDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("should not be reached"))
	}))
	defer server.Close()

	t.Setenv(allowPrivateFetchEnv, "")

	_, _, err := fetchWebContent(context.Background(), server.URL)
	if err == nil {
		t.Fatal("fetchWebContent(loopback) = nil error, want refusal")
	}
	if !strings.Contains(err.Error(), "refusing to fetch non-public address") {
		t.Fatalf("fetchWebContent(loopback) error = %v, want a non-public refusal", err)
	}
}

func TestWebFetchAllowsPrivateDestinationWhenOptedIn(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); !strings.Contains(got, "agent-harness") {
			t.Errorf("User-Agent = %q, want an agent-harness identifier", got)
		}
		_, _ = w.Write([]byte("local content"))
	}))
	defer server.Close()

	t.Setenv(allowPrivateFetchEnv, "1")

	status, content, err := fetchWebContent(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("fetchWebContent() error = %v", err)
	}
	if !strings.Contains(status, "200") {
		t.Fatalf("status = %q, want a 200 status line", status)
	}
	if content != "local content" {
		t.Fatalf("content = %q, want the served body", content)
	}
}

func TestWebFetchBoundsOversizedBodies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("a", 2<<20)))
	}))
	defer server.Close()

	t.Setenv(allowPrivateFetchEnv, "1")

	_, content, err := fetchWebContent(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("fetchWebContent() error = %v", err)
	}
	if len(content) > webFetchMaxOutput+len("\n... (truncated)") {
		t.Fatalf("content length = %d, want it bounded near %d", len(content), webFetchMaxOutput)
	}
	if !strings.HasSuffix(content, "(truncated)") {
		t.Fatalf("content does not end with a truncation marker: %q", content[len(content)-32:])
	}
}

func TestWebFetchStopsRedirectLoops(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, server.URL, http.StatusFound)
	}))
	defer server.Close()

	t.Setenv(allowPrivateFetchEnv, "1")

	_, _, err := fetchWebContent(context.Background(), server.URL)
	if err == nil {
		t.Fatal("fetchWebContent(redirect loop) = nil error, want a redirect limit error")
	}
	if !strings.Contains(err.Error(), "stopped after") {
		t.Fatalf("redirect error = %v, want a redirect-limit message", err)
	}
}

func TestRefusePrivateHostRejectsLocalNames(t *testing.T) {
	ctx := context.Background()
	for _, host := range []string{"localhost", "127.0.0.1", "169.254.169.254", "10.0.0.1", "::1"} {
		if err := refusePrivateHost(ctx, host); err == nil {
			t.Errorf("refusePrivateHost(%q) = nil, want a refusal", host)
		}
	}
	// Public IP literals need no DNS and keep the check from failing open.
	for _, host := range []string{"1.1.1.1", "8.8.8.8"} {
		if err := refusePrivateHost(ctx, host); err != nil {
			t.Errorf("refusePrivateHost(%q) = %v, want nil", host, err)
		}
	}
}

func TestWebFetchToolValidatesInput(t *testing.T) {
	valid := WebFetchTool.ValidateInput(map[string]any{"url": "https://example.com"}, toolsContextForTest())
	if !valid.Valid {
		t.Fatalf("https URL rejected: %s", valid.Message)
	}
	for _, url := range []string{"", "ftp://example.com", "https://user:pass@example.com"} {
		got := WebFetchTool.ValidateInput(map[string]any{"url": url}, toolsContextForTest())
		if got.Valid {
			t.Errorf("ValidateInput(%q) valid = true, want false", url)
		}
	}
}

func TestWebFetchStatusIsPreserved(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusNotFound)
	}))
	defer server.Close()

	t.Setenv(allowPrivateFetchEnv, "1")

	status, content, err := fetchWebContent(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("fetchWebContent() error = %v", err)
	}
	if !strings.Contains(status, "404") {
		t.Fatalf("status = %q, want the 404 status line", status)
	}
	if !strings.Contains(content, "gone") {
		t.Fatalf("content = %q, want the response body", content)
	}
}

// toolsContextForTest returns the zero context; ValidateInput must not
// depend on any runtime state.
func toolsContextForTest() tools.Context {
	return tools.Context{}
}
