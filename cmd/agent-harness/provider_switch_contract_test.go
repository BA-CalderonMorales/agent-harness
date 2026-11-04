package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// Exercise the same login commit and request path in both directions.
// The transport is in-memory: no credentials, network or paid inference.
func TestLocalOpenRouterRoundTrip(t *testing.T) {
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", t.TempDir())
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	const hostedModel = "test/switch-contract:free"
	// Hosted discovery is cached by endpoint across clients. Give this
	// in-memory server its own identity, including across -count runs.
	hostedEndpoint := "https://openrouter.ai/" + filepath.Base(t.TempDir())
	calls := 0
	http.DefaultTransport = lemonadeTransport(func(r *http.Request) (*http.Response, error) {
		local := r.URL.Host == "127.0.0.1:13305"
		if !local && r.URL.Host != "openrouter.ai" {
			t.Errorf("unexpected endpoint: %s", r.URL)
		}
		wantAuth := "Bearer test-router-key"
		if local {
			wantAuth = ""
		}
		if got := r.Header.Get("Authorization"); got != wantAuth {
			t.Errorf("%s auth=%q, want %q", r.URL, got, wantAuth)
		}
		body := ""
		switch {
		case strings.HasSuffix(r.URL.Path, "/models"):
			model, window := hostedModel, 65536
			if local {
				model, window = config.DefaultLemonadeModel, 8192
			}
			body = fmt.Sprintf(`{"data":[{"id":%q,"context_length":%d}]}`, model, window)
		case strings.HasSuffix(r.URL.Path, "/chat/completions"):
			calls++
			body = "data: {\"choices\":[{\"delta\":{\"content\":\"ready\"}}]}\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"
		default:
			t.Errorf("unexpected request: %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "lemonade", EndpointURL: config.DefaultLemonadeEndpoint, ContextLength: 8192, MaxTokens: 4096}, config.DefaultLemonadeModel)
	app.tuiApp = tui.NewApp()
	for _, step := range []struct {
		provider, key, model string
		window               int
	}{
		{"lemonade", "", config.DefaultLemonadeModel, 8192},
		{"openrouter", "test-router-key", hostedModel, 65536},
		{"lemonade", "", config.DefaultLemonadeModel, 8192},
	} {
		app.config.EndpointPinned = true
		app.config.EndpointURL = config.DefaultLemonadeEndpoint
		if step.provider == "openrouter" {
			app.config.EndpointURL = hostedEndpoint
		}
		app.completeLogin(step.provider, step.key, step.model, app.tuiApp)
		if app.config.Provider != step.provider || app.session.Model != step.model {
			t.Fatalf("login did not commit selected provider/model")
		}
		if app.config.ContextLength != step.window || app.loop.Config.BlockingTokenLimit != step.window {
			t.Fatalf("%s inherited another model's context: %d", step.provider, app.config.ContextLength)
		}
		stream, err := app.client.Stream(context.Background(), llm.Request{Model: step.model, MaxTokens: 16})
		if err != nil {
			t.Fatal(err)
		}
		var answer strings.Builder
		for event := range stream {
			switch e := event.(type) {
			case types.LLMTextDelta:
				answer.WriteString(e.Delta)
			case types.LLMError:
				t.Fatalf("%s stream failed: %v", step.provider, e.Error)
			}
		}
		if answer.String() != "ready" {
			t.Fatalf("%s did not stream expected answer: %q", step.provider, answer.String())
		}
	}
	if calls != 3 {
		t.Fatalf("chat requests=%d, want 3", calls)
	}
	if got := app.storedModelForProvider("openrouter"); got != hostedModel {
		t.Fatalf("hosted model forgotten: %s", got)
	}
}
