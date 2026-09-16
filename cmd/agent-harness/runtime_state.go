package main

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
)

// rebuildLLMClient reconstructs client, updates models, and re-probes provider readiness.
func (app *App) rebuildLLMClient() {
	app.client = llm.NewHTTPClientWithBaseURLTimeout(app.config.Provider, app.config.APIKey, app.config.EndpointURL, app.config.HTTPTimeout)
	if app.loop != nil {
		app.loop.Client = app.client
		app.loop.Config.StreamIdleTimeout = app.config.StreamIdleTimeout
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

// persistUserSettings writes the user's runtime preferences to the user
// config layer (~/.agent-harness/settings.json). API keys stay out: they
// belong to the encrypted credential store.
func (app *App) persistUserSettings() {
	values := map[string]interface{}{
		"temperature":     app.config.Temperature, // 0.0 is a valid choice
		"permission_mode": app.config.PermissionMode.String(),
		"perm_read":       app.config.PermRead,
		"perm_write":      app.config.PermWrite,
		"perm_delete":     app.config.PermDelete,
		"perm_execute":    app.config.PermExecute,
		"tagline":         app.config.Tagline, // blank is meaningful: hide the tagline
	}
	// The user layer is a delta over the tracked project config, so a
	// value that carries no information must not be written. Persisting a
	// zero or empty froze it as an override: context_length 0 clobbered
	// the project's context window, and an empty endpoint/model clobbered
	// the project's provider wiring.
	//
	// The provider carries the same rule and it is the costly one to miss:
	// an empty string is dropped on read, so a blank write erased the
	// user's provider and sent the next boot to the default — `local` and
	// the login wall the dead local probe raises behind it. It could not
	// even be repaired from the app, because the blank write repeated.
	if app.config.Provider != "" {
		values["provider"] = app.config.Provider
	}
	if app.config.Runtime != "" {
		values["runtime"] = app.config.Runtime
	}
	if app.config.Effort != "" {
		values["reasoning_effort"] = app.config.Effort
	}
	if app.config.EndpointURL != "" {
		values["endpoint_url"] = app.config.EndpointURL
	}
	if app.config.Model != "" {
		values["model"] = app.config.Model
	}
	if app.config.Theme != "" {
		values["theme"] = app.config.Theme
	}
	if app.config.ContextLength > 0 {
		values["context_length"] = app.config.ContextLength
	}
	if app.config.MaxTokens > 0 {
		values["max_tokens"] = app.config.MaxTokens
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
	app.persistUserSettings()
}

// getPermissionsReport formats active permissions and modes.
