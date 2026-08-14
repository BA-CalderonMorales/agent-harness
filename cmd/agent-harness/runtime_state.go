package main

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
)

// rebuildLLMClient reconstructs client, updates models, and re-probes provider readiness.
func (app *App) rebuildLLMClient() {
	app.client = llm.NewHTTPClientWithBaseURL(app.config.Provider, app.config.APIKey, app.config.EndpointURL)
	if app.loop != nil {
		app.loop.Client = app.client
	}
	if app.tuiApp != nil {
		app.tuiApp.SetModels(app.getModelItems())
		app.tuiApp.SetRuntimeContext(app.config.Provider, app.config.Effort, app.cwd)
		prober := llm.NewHTTPProber(app.config.Provider, app.config.APIKey, app.config.EndpointURL)
		app.tuiApp.StartProviderProbe(prober)
	}
}

// nextEffort returns the next effort level in cycle order.
func nextEffort(current string) string {
	for i, level := range config.EffortLevels {
		if level == current {
			return config.EffortLevels[(i+1)%len(config.EffortLevels)]
		}
	}
	return config.EffortLevels[0]
}

// refreshTelemetry pushes context usage and cost numbers to the TUI footer.
func (app *App) refreshTelemetry(tuiApp *tui.App) {
	if tuiApp == nil {
		return
	}
	est := 0
	if app.session != nil {
		est = app.session.EstimateTokens()
	}
	cost := 0.0
	if app.costTracker != nil {
		cost = app.costTracker.GetTotalCost()
	}
	tuiApp.SetTelemetry(est, app.config.ContextLength, cost)
}

// syncModelFields converges app.config.Model and session.Model on the
// session's model (the live, per-request value) when one exists.
func (app *App) syncModelFields() {
	if app.session == nil {
		return
	}
	if app.session.Model != "" {
		app.config.Model = app.session.Model
	} else if app.config.Model != "" {
		app.session.Model = app.config.Model
	}
}

// persistUserSettings writes the user's runtime preferences to the user
// config layer (~/.agent-harness/settings.json). API keys stay out: they
// belong to the encrypted credential store.
func (app *App) persistUserSettings() {
	values := map[string]interface{}{
		"provider":         app.config.Provider,
		"endpoint_url":     app.config.EndpointURL,
		"runtime":          app.config.Runtime,
		"model":            app.config.Model,
		"context_length":   app.config.ContextLength,
		"temperature":      app.config.Temperature,
		"max_tokens":       app.config.MaxTokens,
		"reasoning_effort": app.config.Effort,
	}
	loader := config.NewLayeredLoader(app.cwd)
	if err := loader.SaveSettings(config.SourceUser, values); err != nil {
		msg := sprintf("Warning: failed to save settings: %v", err)
		if app.tuiApp != nil {
			app.tuiApp.Send(tui.StatusMsg{Text: msg, Type: "warning"})
		}
	}
}

// commitConfigChange persists the current runtime configuration after any
// in-session mutation so provider/model choices survive restarts.
func (app *App) commitConfigChange() {
	app.syncModelFields()
	app.persistUserSettings()
}

// getPermissionsReport formats active permissions and modes.
