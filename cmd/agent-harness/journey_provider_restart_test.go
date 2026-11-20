package main

import (
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
)

// The journey: a user on the default local provider logs in to a hosted
// provider and picks a model, quits, and starts the harness again. The
// provider, model, and endpoint they chose must come back, and the home
// screen must say which provider is active.
func TestJourneyProviderChoiceSurvivesRestart(t *testing.T) {
	journeyHome(t)
	const (
		provider = "openrouter"
		model    = "journey/model-b"
		key      = "sk-journey-test"
	)

	first := bootJourneyApp(t)
	if first.config.Provider == provider {
		t.Fatalf("first boot already on %s; the journey needs a switch", provider)
	}
	tuiApp := wireJourneyTUI(t, first)
	screen := newJourneyScreen(t, tuiApp, 80, 24)

	// The login wizard's completion is the path a user's choice takes.
	first.completeLogin(provider, key, model, tuiApp)
	screen.Pump()
	wantEndpoint := first.config.EndpointURL
	if wantEndpoint == "" {
		t.Fatal("login committed no endpoint")
	}
	if first.config.Provider != provider || first.session.Model != model {
		t.Fatalf("login committed %s/%s, want %s/%s", first.config.Provider, first.session.Model, provider, model)
	}
	screen.AssertContains("Provider: " + provider)

	// Quit: drop the app. Restart on the same home and workspace.
	first = nil
	second := bootJourneyApp(t)

	if second.config.Provider != provider {
		t.Fatalf("provider after restart = %q, want %q", second.config.Provider, provider)
	}
	if second.config.Model != model {
		t.Fatalf("config model after restart = %q, want %q", second.config.Model, model)
	}
	if second.session.Model != model {
		t.Fatalf("session model after restart = %q, want %q", second.session.Model, model)
	}
	if second.config.EndpointURL != wantEndpoint {
		t.Fatalf("endpoint after restart = %q, want %q", second.config.EndpointURL, wantEndpoint)
	}
	if second.config.APIKey != key {
		t.Fatal("API key did not come back from the credential store")
	}
	if config.IsLocalProvider(second.config.Provider) {
		t.Fatal("restart fell back to a local provider")
	}

	// The restarted home screen names the provider at a normal pane.
	restarted := newJourneyScreen(t, wireJourneyTUI(t, second), 80, 24)
	restarted.Pump()
	restarted.Resize(80, 24)
	restarted.AssertFitsWidth(80)
	// The dashboard's identity line reads "<workspace> · provider/model".
	restarted.AssertContains(provider + "/" + model)
}
