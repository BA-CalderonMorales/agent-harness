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
