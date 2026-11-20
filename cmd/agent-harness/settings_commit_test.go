package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
)

// readUserSettings returns the keys of the user layer's settings.json.
func readUserSettings(t *testing.T) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(config.ConfigHome(), "settings.json"))
	if err != nil {
		t.Fatalf("read user settings: %v", err)
	}
	var values map[string]interface{}
	if err := json.Unmarshal(data, &values); err != nil {
		t.Fatalf("parse user settings: %v", err)
	}
	return values
}

// An environment pin is a per-session override. Saving an unrelated
// setting must not freeze the pinned provider, model, or endpoint into
// the user layer, where they would outlive the environment.
func TestCommitConfigChangeSkipsEnvPinnedValues(t *testing.T) {
	journeyHome(t)
	t.Setenv("AH_PROVIDER", "openrouter")
	t.Setenv("AH_MODEL", "journey/model-b")
	t.Setenv("AH_ENDPOINT_URL", "https://pinned.example/v1")
	t.Setenv("AH_API_KEY", "sk-pinned-test")

	app := bootJourneyApp(t)
	if app.config.Provider != "openrouter" || app.config.Model != "journey/model-b" || app.config.EndpointURL != "https://pinned.example/v1" {
		t.Fatalf("boot did not apply the pins: %s %s %s", app.config.Provider, app.config.Model, app.config.EndpointURL)
	}

	app.config.Theme = "nord"
	app.commitConfigChange()

	saved := readUserSettings(t)
	if saved["theme"] != "nord" {
		t.Fatalf("theme = %v, want nord (the unrelated change must persist)", saved["theme"])
	}
	for _, key := range []string{"provider", "model", "endpoint_url"} {
		if v, ok := saved[key]; ok {
			t.Errorf("env-pinned %s = %v was written to settings.json", key, v)
		}
	}
}

// Without a pin the same fields are the user's choice and persist.
func TestCommitConfigChangePersistsUnpinnedChoices(t *testing.T) {
	journeyHome(t)
	app := bootJourneyApp(t)
	app.config.Provider = "openrouter"
	app.config.Model = "chosen/model"
	app.config.EndpointURL = "https://chosen.example/v1"
	app.commitConfigChange()

	saved := readUserSettings(t)
	if saved["provider"] != "openrouter" || saved["model"] != "chosen/model" || saved["endpoint_url"] != "https://chosen.example/v1" {
		t.Fatalf("unpinned choices not persisted: %v", saved)
	}
}
