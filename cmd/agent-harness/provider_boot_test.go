package main

import (
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
)

// TestProviderSurvivesSettingsReload pins the boot contract the user
// relies on: run `make build && make run` and land back on the provider
// you were last using. The provider is persisted as part of the user
// settings delta, and boot resolves its config from the layered loader,
// so a fresh load returning the chosen provider is that guarantee.
func TestProviderSurvivesSettingsReload(t *testing.T) {
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", t.TempDir())
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "anthropic", Model: "claude-sonnet-4-5"}, "claude-sonnet-4-5")
	app.cwd = t.TempDir()
	app.commitConfigChange()

	loaded, err := config.NewLayeredLoader(app.cwd).Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Provider != "anthropic" {
		t.Fatalf("provider = %q after reload, want anthropic", loaded.Provider)
	}
	if loaded.Model != "claude-sonnet-4-5" {
		t.Fatalf("model = %q after reload, want the model chosen with that provider", loaded.Model)
	}
}
