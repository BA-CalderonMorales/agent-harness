package main

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
)

type lemonadeTransport func(*http.Request) (*http.Response, error)

func (f lemonadeTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestLemonadeDiscoveryLoginAndRestart(t *testing.T) {
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", t.TempDir())
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	ids := `{"data":[{"id":"qwen3.5-4b-FLM"},{"id":"next-local-model"}]}`
	http.DefaultTransport = lemonadeTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != config.DefaultLemonadeEndpoint+"/models" {
			t.Fatalf("wrong discovery endpoint: %s", r.URL)
		}
		if r.Header.Get("Authorization") != "" {
			t.Fatal("local models must not receive a hosted API key")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(ids)), Header: make(http.Header)}, nil
	})
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "openai", EndpointURL: config.DefaultEndpointForProvider("openai"), APIKey: "hosted-key", ContextLength: 8192, MaxTokens: 4096}, "cloud-model")
	items, err := app.wizardModels("lemonade", "")
	if err != nil || len(items) != 2 || !items[0].IsDefault {
		t.Fatalf("live discovery: %v %v", items, err)
	}
	app.tuiApp = tui.NewApp()
	app.completeLogin("lemonade", "", config.DefaultLemonadeModel, app.tuiApp)
	if app.config.EndpointURL != config.DefaultLemonadeEndpoint || app.config.Runtime != "lemonade" {
		t.Fatalf("wrong committed connection: %+v", app.config)
	}
	if app.loop.Config.BlockingTokenLimit != app.config.ContextLength {
		t.Fatal("login lost the model context budget")
	}
	if app.config.HTTPTimeout != config.DefaultLocalHTTPTimeout {
		t.Fatal("local timeout defaults not applied")
	}
	if got := app.storedModelForProvider("lemonade"); got != config.DefaultLemonadeModel {
		t.Fatalf("model not retained: %s", got)
	}
	// The ordinary loader must restore the connection without launcher environment.
	restored, err := config.NewLayeredLoader(app.cwd).Load()
	if err != nil {
		t.Fatal(err)
	}
	if restored.Provider != "lemonade" || restored.EndpointURL != config.DefaultLemonadeEndpoint || restored.Model != config.DefaultLemonadeModel {
		t.Fatalf("restart lost connection: %s %s %s", restored.Provider, restored.EndpointURL, restored.Model)
	}
	// Pulling another model must be reflected immediately, not after a 5-minute cache.
	ids = `{"data":[{"id":"new-model"}]}`
	items, err = app.wizardModels("lemonade", "")
	if err != nil || len(items) != 1 || items[0].ID != "new-model" {
		t.Fatalf("discovery is stale: %v %v", items, err)
	}
	ids = `{"data":[]}`
	items, err = app.wizardModels("lemonade", "")
	if err == nil || len(items) != 0 {
		t.Fatal("empty endpoint must not advertise an uninstalled default")
	}
}

func TestLemonadeSavedEndpointAndDefaults(t *testing.T) {
	app := &App{config: &config.LayeredConfig{Provider: "lemonade", EndpointURL: "http://127.0.0.1:14444/v1"}}
	if app.wizardEndpoint("lemonade") != app.config.EndpointURL {
		t.Fatal("reconnecting lost the saved endpoint")
	}
	if app.wizardEndpoint("flm") != config.DefaultEndpointForProvider("flm") {
		t.Fatal("direct FLM inherited Lemonade's endpoint")
	}
	c := llm.NewHTTPClient("lemonade", "")
	if c.BaseURL != config.DefaultLemonadeEndpoint || !config.IsLocalProvider("lemonade") {
		t.Fatal("Lemonade must use local defaults")
	}
}
