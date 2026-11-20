package main

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
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
	// The bar is context usage, not transcript usage: the system prompt
	// and tool schemas are sent on every request and dominated the
	// count on hosted turns (the pane read 2k while the provider billed
	// ~10k). Count the fixed prompt before painting the bar.
	if app.toolRegistry != nil && app.session != nil {
		est += agent.EstimatePromptTokens(app.buildSystemPrompt(), app.enabledToolsForMode())
	}
	cost := 0.0
	if app.costTracker != nil {
		cost = app.costTracker.GetTotalCost()
	}
	tuiApp.SetTelemetry(est, app.config.ContextLength, cost)
}

// getPermissionsReport formats active permissions and modes.
