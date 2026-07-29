package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPProber_ProbeOpenAICompatible(t *testing.T) {
	tests := []struct {
		name          string
		handler       http.HandlerFunc
		wantReadiness ProviderReadiness
		wantMsgSubstr string
	}{
		{
			name: "successful probe with models",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":[{"id":"gpt-4"},{"id":"gpt-3.5"}]}`))
			},
			wantReadiness: ProviderReady,
			wantMsgSubstr: "2 models available",
		},
		{
			name: "successful probe with no models",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":[]}`))
			},
			wantReadiness: ProviderWarning,
			wantMsgSubstr: "no models available",
		},
		{
			name: "unauthorized",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			wantReadiness: ProviderMisconfigured,
			wantMsgSubstr: "invalid API key",
		},
		{
			name: "forbidden",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			},
			wantReadiness: ProviderMisconfigured,
			wantMsgSubstr: "invalid API key",
		},
		{
			name: "not found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantReadiness: ProviderWarning,
			wantMsgSubstr: "endpoint not found",
		},
		{
			name: "server error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantReadiness: ProviderWarning,
			wantMsgSubstr: "HTTP 500",
		},
		{
			name: "invalid JSON",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`invalid json`))
			},
			wantReadiness: ProviderWarning,
			wantMsgSubstr: "invalid response format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			prober := &HTTPProber{
				BaseURL:  server.URL,
				APIKey:   "test-key",
				Provider: "openai",
				Timeout:  1 * time.Second,
			}

			ctx := context.Background()
			readiness, msg := prober.Probe(ctx)

			if readiness != tt.wantReadiness {
				t.Errorf("readiness = %v, want %v", readiness, tt.wantReadiness)
			}
			if tt.wantMsgSubstr != "" && msg == "" {
				t.Errorf("expected message containing %q, got empty", tt.wantMsgSubstr)
			}
		})
	}
}

func TestHTTPProber_ProbeOllama(t *testing.T) {
	tests := []struct {
		name          string
		handler       http.HandlerFunc
		wantReadiness ProviderReadiness
		wantMsgSubstr string
	}{
		{
			name: "successful probe with models",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"models":[{"name":"llama2"},{"name":"codellama"}]}`))
			},
			wantReadiness: ProviderReady,
			wantMsgSubstr: "2 models available",
		},
		{
			name: "successful probe with no models",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"models":[]}`))
			},
			wantReadiness: ProviderWarning,
			wantMsgSubstr: "no models pulled",
		},
		{
			name: "server error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantReadiness: ProviderWarning,
			wantMsgSubstr: "HTTP 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			prober := &HTTPProber{
				BaseURL:  server.URL,
				APIKey:   "",
				Provider: "ollama",
				Timeout:  1 * time.Second,
			}

			ctx := context.Background()
			readiness, msg := prober.Probe(ctx)

			if readiness != tt.wantReadiness {
				t.Errorf("readiness = %v, want %v", readiness, tt.wantReadiness)
			}
			if tt.wantMsgSubstr != "" && msg == "" {
				t.Errorf("expected message containing %q, got empty", tt.wantMsgSubstr)
			}
		})
	}
}

func TestHTTPProber_ProbeTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	prober := &HTTPProber{
		BaseURL:  server.URL,
		APIKey:   "test-key",
		Provider: "openai",
		Timeout:  100 * time.Millisecond,
	}

	ctx := context.Background()
	readiness, msg := prober.Probe(ctx)

	if readiness != ProviderUnavailable {
		t.Errorf("readiness = %v, want %v", readiness, ProviderUnavailable)
	}
	if msg != "provider timeout" {
		t.Errorf("msg = %q, want %q", msg, "provider timeout")
	}
}

func TestHTTPProber_ProbeConnectionError(t *testing.T) {
	prober := &HTTPProber{
		BaseURL:  "http://localhost:99999", // Invalid port
		APIKey:   "test-key",
		Provider: "openai",
		Timeout:  1 * time.Second,
	}

	ctx := context.Background()
	readiness, msg := prober.Probe(ctx)

	if readiness != ProviderUnavailable {
		t.Errorf("readiness = %v, want %v", readiness, ProviderUnavailable)
	}
	if msg == "" {
		t.Error("expected non-empty error message")
	}
}

func TestProviderReadiness_String(t *testing.T) {
	tests := []struct {
		readiness ProviderReadiness
		want      string
	}{
		{ProviderChecking, "checking"},
		{ProviderReady, "ready"},
		{ProviderWarning, "warning"},
		{ProviderUnavailable, "unavailable"},
		{ProviderMisconfigured, "misconfigured"},
		{ProviderReadiness(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.readiness.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
