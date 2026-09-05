package main

import (
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
)

// Per-provider model retention: logging in to a provider with a
// model, then coming back to that provider, must pre-select the
// model that was used — not the catalog default. The keys already
// survived provider switches; the model choice used to vanish
// (desktop and mobile).

func TestLoginRetainsProviderModel(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", configHome)
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "openai"}, "demo-1.0")
	app.tuiApp = tui.NewApp()

	app.completeLogin("openai", "sk-test", "gpt-4-turbo", app.tuiApp)

	if got := app.storedModelForProvider("openai"); got != "gpt-4-turbo" {
		t.Fatalf("stored model for openai = %q, want the login's model", got)
	}
}

// TestLoginSwitchDoesNotClobberOtherProviderModel: logging in to
// a second provider must not disturb the first provider's retained
// model — the keys already merged; the models now merge too.
func TestLoginSwitchDoesNotClobberOtherProviderModel(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", configHome)
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "openrouter"}, "demo-1.0")
	app.tuiApp = tui.NewApp()

	app.completeLogin("openrouter", "sk-a", "openrouter/model-a", app.tuiApp)
	app.completeLogin("anthropic", "sk-b", "claude-model-b", app.tuiApp)

	if got := app.storedModelForProvider("openrouter"); got != "openrouter/model-a" {
		t.Fatalf("openrouter model clobbered: %q", got)
	}
	if got := app.storedModelForProvider("anthropic"); got != "claude-model-b" {
		t.Fatalf("anthropic model missing: %q", got)
	}
}

// TestLocalProviderModelRetained: local providers keep their model
// choice too — a llama.cpp user's Ornith GGUF re-selects on return.
func TestLocalProviderModelRetained(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", configHome)
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "local"}, "demo-1.0")
	app.tuiApp = tui.NewApp()

	app.completeLogin("local", "", "ornith-1.0-gguf", app.tuiApp)

	if got := app.storedModelForProvider("local"); got != "ornith-1.0-gguf" {
		t.Fatalf("local provider model = %q, want ornith-1.0-gguf", got)
	}
}
