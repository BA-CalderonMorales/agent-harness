package main

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
)

// completeLogin persists the modal login wizard's result: the provider,
// the (masked-input) API key, and the model. The key goes to the
// encrypted store; the provider/model go through the normal config
// commit path. Runs on the event loop via SetLoginHandler.
func (app *App) completeLogin(provider, apiKey, model string, tuiApp *tui.App) {
	if provider == "" {
		return
	}
	app.config.Provider = provider
	app.config.EndpointURL = config.DefaultEndpointForProvider(provider)

	if config.IsLocalProvider(provider) {
		app.config.APIKey = provider
		tuiApp.AddMessage("system", sprintf("Local provider configured. Model: %s", model))
	} else if apiKey != "" {
		app.config.APIKey = apiKey
		credManager := config.NewCredentialManager()
		secureCfg := &config.SecureConfig{
			Provider: provider,
			APIKey:   apiKey,
			Model:    model,
		}
		if err := credManager.SaveSecure(secureCfg); err != nil {
			tuiApp.AddMessage("system", sprintf("[!] Failed to save credentials: %v", err))
		} else {
			tuiApp.AddMessage("system", sprintf("Credentials saved (encrypted at rest, file mode 0600; machine-local key at %s). Store: %s. To source the key from a secrets manager instead, set api_key to a secret://env|file|cmd reference in agent-harness.yml.", config.MachineKeyPath(), config.SecureConfigPath()))
		}
	} else if app.config.APIKey != "" {
		// No key typed and one is already configured: retain it. Only
		// the provider/model in the store are refreshed so a provider
		// switch survives restarts without re-authenticating. The
		// message must not echo any key material (not even the masked
		// hint): chat messages persist to session files and exports.
		tuiApp.AddMessage("system", sprintf("Using stored API key. Provider: %s", provider))
		credManager := config.NewCredentialManager()
		if secureCfg, err := credManager.LoadSecure(); err == nil {
			secureCfg.Provider = provider
			secureCfg.Model = model
			_ = credManager.SaveSecure(secureCfg)
		}
	} else {
		tuiApp.AddMessage("system", "No API key entered; provider left misconfigured.")
	}

	if model == "" {
		model = getDefaultModel(provider)
	}
	app.config.Model = model
	if app.session != nil {
		app.session.Model = model
	}
	if app.costTracker != nil {
		app.costTracker.SetModel(model)
	}
	app.commitConfigChange()

	// Recreate the LLM client and refresh the TUI state.
	app.client = llm.NewHTTPClientWithBaseURL(app.config.Provider, app.config.APIKey, app.config.EndpointURL)
	app.loop = agent.NewLoop(app.client)
	tuiApp.SetChatModel(model)
	tuiApp.SetSettings(app.getSettings())
	tuiApp.SetRuntimeContext(app.config.Provider, app.config.Effort, app.cwd)
	tuiApp.SetModels(app.getModelItems())
	tuiApp.AddMessage("system", sprintf("Logged in. Provider: %s | Model: %s", provider, model))

	// Re-probe the new provider.
	prober := llm.NewHTTPProber(app.config.Provider, app.config.APIKey, app.config.EndpointURL)
	tuiApp.StartProviderProbe(prober)
}
