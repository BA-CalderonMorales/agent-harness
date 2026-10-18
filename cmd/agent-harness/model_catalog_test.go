package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
)

// TestFireworksCatalogMatchesLiveAudit pins the audited catalog values:
// glm-5p3-flash advertises a 1M window on the live /v1/models response;
// the old 131072 entry starved the context bar 8x.
func TestFireworksCatalogMatchesLiveAudit(t *testing.T) {
	items := getModelsForProvider("fireworks", "accounts/fireworks/models/glm-5p3-flash")
	byID := make(map[string]int, len(items))
	for _, item := range items {
		byID[item.ID] = item.ContextLen
	}
	for _, id := range []string{
		"accounts/fireworks/models/glm-5p3-flash",
		"accounts/fireworks/models/glm-5p3",
		"accounts/fireworks/models/deepseek-v4-flash-0731",
	} {
		if byID[id] != 1048576 {
			t.Fatalf("%s catalog context = %d, want 1048576 (live audit)", id, byID[id])
		}
	}
	for _, dead := range []string{
		"accounts/fireworks/models/llama-v3p3-70b-instruct",
		"accounts/fireworks/models/mixtral-8x22b-instruct",
	} {
		if _, ok := byID[dead]; ok {
			t.Fatalf("%s is no longer served live and must not be selectable", dead)
		}
	}
}

// TestApplyModelContextPrefersLive pins the precedence: the endpoint's
// advertised context_length wins over the catalog; when the live list
// fails or stays silent, the catalog is the fallback.
func TestApplyModelContextPrefersLive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "accounts/fireworks/models/glm-5p3-flash", "context_length": 1048576},
			},
		})
	}))
	defer srv.Close()

	app := &App{config: &config.LayeredConfig{Provider: "fireworks"}}
	app.client = llm.NewHTTPClientWithBaseURL("fireworks", "key", srv.URL)

	app.applyModelContext("accounts/fireworks/models/glm-5p3-flash")
	if app.config.ContextLength != 1048576 {
		t.Fatalf("live-advertised context not applied: %d", app.config.ContextLength)
	}

	// Live endpoint silent about the model: catalog fallback.
	app.config.ContextLength = 1
	app.applyModelContext("accounts/fireworks/models/glm-5p3")
	if app.config.ContextLength != 1048576 {
		t.Fatalf("catalog fallback not applied: %d", app.config.ContextLength)
	}
}

// TestEnsureModelFitsProviderLocalUsesLiveList covers the user's repro:
// boot local with a stranded fireworks model and the mode line reads
// "glm-5p3-flash · local" — the live list resets it to the local
// default, while a model the endpoint serves stays. Hosted providers
// validate against the static catalog instead.
func TestEnsureModelFitsProviderLocalUsesLiveList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "deepreinforce-ai/Ornith-1.0-9B-GGUF", "context_length": 8192},
			},
		})
	}))
	defer srv.Close()

	app := &App{client: llm.NewHTTPClientWithBaseURL("local", "", srv.URL)}
	cfg := &config.LayeredConfig{Provider: "local", Model: "accounts/fireworks/models/glm-5p3-flash"}

	previous, reset := app.ensureModelFitsProvider(cfg, cfg.Model)
	if !reset {
		t.Fatal("stranded fireworks model on local provider must reset")
	}
	if previous != "accounts/fireworks/models/glm-5p3-flash" {
		t.Fatalf("previous = %q", previous)
	}
	if cfg.Model != config.DefaultModel {
		t.Fatalf("model = %q, want the local default", cfg.Model)
	}

	// A model the endpoint actually serves stays.
	cfg.Model = "deepreinforce-ai/Ornith-1.0-9B-GGUF"
	if _, reset = app.ensureModelFitsProvider(cfg, cfg.Model); reset {
		t.Fatal("live-served model must not reset")
	}
}

// The user's repro: a Fireworks model chosen from the live picker is not in
// the curated static catalog, so boot reset it to the provider default and
// the saved choice never took effect. With the endpoint reachable and
// serving the model, the choice must be kept.
func TestEnsureModelFitsProviderKeepsLiveHostedModel(t *testing.T) {
	const chosen = "accounts/fireworks/models/deepseek-v4p1-flash"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": chosen, "context_length": 1048576},
			},
		})
	}))
	defer srv.Close()

	app := &App{client: llm.NewHTTPClientWithBaseURL("fireworks", "key", srv.URL)}
	cfg := &config.LayeredConfig{Provider: "fireworks", Model: chosen}

	if previous, reset := app.ensureModelFitsProvider(cfg, cfg.Model); reset {
		t.Fatalf("live-served hosted model was reset: %q -> %q", previous, cfg.Model)
	}
	if cfg.Model != chosen {
		t.Fatalf("model = %q, want the chosen %q", cfg.Model, chosen)
	}
}

// The stranding case still resets: the endpoint is reachable and does not
// serve the configured model, so it is a leftover from another provider.
func TestEnsureModelFitsProviderResetsHostedModelEndpointRejects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "accounts/fireworks/models/glm-5p3-flash", "context_length": 1048576},
			},
		})
	}))
	defer srv.Close()

	app := &App{client: llm.NewHTTPClientWithBaseURL("fireworks", "key", srv.URL)}
	cfg := &config.LayeredConfig{Provider: "fireworks", Model: "accounts/fireworks/models/not-served"}

	previous, reset := app.ensureModelFitsProvider(cfg, cfg.Model)
	if !reset {
		t.Fatal("a hosted model the endpoint does not serve must reset")
	}
	if previous != "accounts/fireworks/models/not-served" {
		t.Fatalf("previous = %q", previous)
	}
	if cfg.Model != config.DefaultModelForProvider("fireworks") {
		t.Fatalf("model = %q, want the fireworks default", cfg.Model)
	}
}

// No endpoint and no catalog entry: the curated catalog is not the
// provider's full serving list, so absence is not evidence. A model chosen
// from the live picker must survive boot instead of being clobbered to the
// default. A catalog-known model is kept by the offline short circuit.
func TestEnsureModelFitsProviderKeepsHostedModelWithoutEvidence(t *testing.T) {
	app := &App{} // no client: nothing to ask
	cfg := &config.LayeredConfig{Provider: "fireworks", Model: "accounts/fireworks/models/deepseek-v4p1-flash"}

	if previous, reset := app.ensureModelFitsProvider(cfg, cfg.Model); reset {
		t.Fatalf("reset without endpoint evidence: %q -> %q", previous, cfg.Model)
	}
	if cfg.Model != "accounts/fireworks/models/deepseek-v4p1-flash" {
		t.Fatalf("model = %q, want it unchanged", cfg.Model)
	}

	cfg.Model = "accounts/fireworks/models/glm-5p3-flash" // in the curated catalog
	if _, reset := app.ensureModelFitsProvider(cfg, cfg.Model); reset {
		t.Fatal("a catalog-known model must not reset")
	}
}
