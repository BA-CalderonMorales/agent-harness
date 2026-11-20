package main

import "testing"

// Open product question, pinned here as today's behavior: with the
// provider pinned by the environment, an in-session login to another
// provider is NOT persisted. The session uses it; the user file keeps
// its own provider, endpoint, model and limits, and the env wins on the
// next boot anyway.
func TestInSessionLoginWhileProviderPinnedPersistsNoProviderValues(t *testing.T) {
	journeyHome(t)
	before := seedCompatUserSettings(t)
	t.Setenv("AH_PROVIDER", "openrouter")
	t.Setenv("AH_API_KEY", "sk-pinned-test")

	app := bootJourneyApp(t)
	tuiApp := wireJourneyTUI(t, app)
	screen := newJourneyScreen(t, tuiApp, 80, 24)
	app.completeLogin("anthropic", "sk-login-test", "claude-model-x", tuiApp)
	screen.Pump() // let the provider probe finish before the offline transport is restored
	if app.config.Provider != "anthropic" {
		t.Fatalf("session provider = %q, want the login's choice", app.config.Provider)
	}

	assertDerivedKeysUnchanged(t, before, readUserSettings(t))
}

// A blank provider under a pin must not panic or write a provider
// (or any provider-derived value) over the user's file.
func TestPersistWithBlankPinnedProviderLeavesProviderValuesAlone(t *testing.T) {
	journeyHome(t)
	before := seedCompatUserSettings(t)
	t.Setenv("AH_PROVIDER", "openrouter")
	t.Setenv("AH_API_KEY", "sk-pinned-test")

	app := bootJourneyApp(t)
	app.config.Provider = ""
	app.commitConfigChange()

	assertDerivedKeysUnchanged(t, before, readUserSettings(t))
}
