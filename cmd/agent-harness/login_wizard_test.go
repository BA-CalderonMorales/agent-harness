package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
)

// TestWizardModelsLiveList is the lightning bolt: the wizard's model
// step shows the actual models the candidate endpoint serves, with the
// provider's pinned default marked.
func TestWizardModelsLiveList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/models") {
			t.Fatalf("probe hit %s, want /models", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data": []map[string]any{
				{"id": "gpt-4o", "object": "model"},
				{"id": "demo-1.0", "object": "model"},
			},
		})
	}))
	defer server.Close()

	app := &App{config: &config.LayeredConfig{EndpointURL: server.URL, EndpointPinned: true}}
	items, err := app.wizardModels("openai", "sk-test")
	if err != nil {
		t.Fatalf("wizardModels error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("wizardModels returned %d items, want the 2 live models", len(items))
	}
	ids := map[string]bool{}
	pins := 0
	for _, item := range items {
		ids[item.ID] = true
		if item.IsDefault {
			pins++
			if item.ID != getDefaultModel("openai") {
				t.Fatalf("IsDefault marked on %q, want the pinned default %q", item.ID, getDefaultModel("openai"))
			}
		}
	}
	if !ids["gpt-4o"] || !ids["demo-1.0"] {
		t.Fatalf("live model ids = %v, want gpt-4o and demo-1.0", ids)
	}
	if pins != 1 {
		t.Fatalf("IsDefault pins = %d, want exactly 1 (%q)", pins, getDefaultModel("openai"))
	}
}

// TestWizardModelsFailingProbeFallsBackToCatalog: a dead endpoint must
// surface an honest error next to the static catalog, never a silent
// default.
func TestWizardModelsFailingProbeFallsBackToCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer server.Close()

	app := &App{config: &config.LayeredConfig{EndpointURL: server.URL, EndpointPinned: true}}
	items, err := app.wizardModels("openai", "sk-test")
	if err == nil {
		t.Fatal("failing probe must return an error")
	}
	if len(items) == 0 {
		t.Fatal("failing probe must still return the static catalog")
	}
	if items[0].Provider != "openai" {
		t.Fatalf("catalog items provider = %q, want openai", items[0].Provider)
	}
}

// TestWizardEndpointPinning: an env-pinned endpoint survives the login;
// otherwise the provider default applies - the probe and completeLogin
// must agree on the endpoint or the green check would lie.
func TestWizardEndpointPinning(t *testing.T) {
	app := &App{config: &config.LayeredConfig{EndpointURL: "http://127.0.0.1:9999/v1", EndpointPinned: true}}
	if got := app.wizardEndpoint("openai"); got != "http://127.0.0.1:9999/v1" {
		t.Fatalf("pinned endpoint = %q, want the env-pinned URL", got)
	}

	app = &App{config: &config.LayeredConfig{}}
	if got := app.wizardEndpoint("openai"); got != config.DefaultEndpointForProvider("openai") {
		t.Fatalf("unpinned endpoint = %q, want the provider default", got)
	}
}
