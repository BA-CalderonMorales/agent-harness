package main

import (
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
)

// wizardEndpoint returns the endpoint the wizard commits for a provider:
// an env-pinned AH_ENDPOINT_URL survives the login, anything else follows
// the provider default. The wizard's live model probe must test exactly
// this endpoint or its green check would lie about the app's state.
func (app *App) wizardEndpoint(provider string) string {
	if app.config.EndpointPinned {
		return app.config.EndpointURL
	}
	return config.DefaultEndpointForProvider(provider)
}

// wizardModels returns the model list the login wizard's model step
// shows for a candidate provider+key: a live ListModels() call against
// the endpoint the login will commit is the verified-connection proof,
// and the static catalog is the honest fallback when the probe fails
// (surfaced as an error, never a silent default). The probe is bounded:
// a dead endpoint must not hold the wizard hostage.
func (app *App) wizardModels(provider, apiKey string) ([]tui.ModelItem, error) {
	client := llm.NewHTTPClientWithBaseURL(provider, apiKey, app.wizardEndpoint(provider))
	client.HTTPClient.Timeout = 3 * time.Second

	ids, err := client.ListModels()
	if err != nil {
		return getModelsForProvider(provider, getDefaultModel(provider)), err
	}
	if len(ids) == 0 {
		return getModelsForProvider(provider, getDefaultModel(provider)), nil
	}
	defaultID := getDefaultModel(provider)
	items := make([]tui.ModelItem, 0, len(ids))
	for _, id := range ids {
		items = append(items, tui.ModelItem{
			ID:        id,
			Name:      id,
			Provider:  provider,
			IsDefault: id == defaultID,
		})
	}
	return items, nil
}

// completeLogin persists the modal login wizard's result: the provider,
// the (masked-input) API key, and the model. The key goes to the
// encrypted store; the provider/model go through the normal config
// commit path. Runs on the event loop via SetLoginHandler.
func (app *App) completeLogin(provider, apiKey, model string, tuiApp *tui.App) {
	if provider == "" {
		return
	}
	app.config.Provider = provider
	app.config.EndpointURL = app.wizardEndpoint(provider)
	app.applyLoginCredentials(provider, apiKey, model, tuiApp)

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
	app.client = llm.NewHTTPClientWithBaseURLTimeout(app.config.Provider, app.config.APIKey, app.config.EndpointURL, app.config.HTTPTimeout)
	app.loop = agent.NewLoop(app.client)
	tuiApp.SetChatModel(model)
	tuiApp.SetSettings(app.getSettings())
	tuiApp.SetRuntimeContext(app.config.Provider, app.config.Effort, app.cwd)
	tuiApp.SetModels(app.getModelItems())
	tuiApp.AddMessage("system", sprintf("Logged in. Provider: %s | Model: %s", provider, model))

	// Re-probe the new provider.
	prober := llm.NewHTTPProber(app.config.Provider, app.config.APIKey, app.config.EndpointURL)
	tuiApp.StartProviderProbe(prober)

	// Land the user in chat, ready to type: the first-run happy path
	// never strands them on the home screen after authenticating.
	tuiApp.Send(tui.LoginCompletedMsg{})
}

// applyLoginCredentials resolves the API key for the login result —
// typed key, local dummy, stored key, or an honest "none" — and
// persists it. One policy per early return, in priority order.
func (app *App) applyLoginCredentials(provider, apiKey, model string, tuiApp *tui.App) {
	if config.IsLocalProvider(provider) {
		app.config.APIKey = provider
		tuiApp.AddMessage("system", sprintf("Local provider configured. Model: %s", model))
		return
	}

	if apiKey != "" {
		app.config.APIKey = apiKey
		app.saveTypedCredentials(provider, apiKey, model, tuiApp)
		return
	}

	storedKey, ok := app.storedKeyForProvider(provider, config.NewCredentialManager())
	if ok {
		// No key typed: use a key that can actually authenticate THIS
		// provider (env pin, encrypted store, or the config key when it
		// was already authenticating it). The message must not echo any
		// key material (not even the masked hint): chat messages
		// persist to session files and exports.
		app.config.APIKey = storedKey
		tuiApp.AddMessage("system", sprintf("Using stored API key. Provider: %s", provider))
		app.refreshStoredModel(provider, model)
		return
	}

	// No usable key for this provider: say so instead of sending a
	// stale or dummy credential as auth (a local dummy or another
	// provider's key 401s with "missing authentication header").
	app.config.APIKey = ""
	tuiApp.AddMessage("system", sprintf("[!] No stored API key for %s. Run /login and paste the key, or set AH_API_KEY.", provider))
}

// saveTypedCredentials stores a freshly typed key in the encrypted
// credential store.
func (app *App) saveTypedCredentials(provider, apiKey, model string, tuiApp *tui.App) {
	credManager := config.NewCredentialManager()
	secureCfg := &config.SecureConfig{
		Provider: provider,
		APIKey:   apiKey,
		Model:    model,
	}
	if err := credManager.SaveSecure(secureCfg); err != nil {
		tuiApp.AddMessage("system", sprintf("[!] Failed to save credentials: %v", err))
		return
	}
	tuiApp.AddMessage("system", sprintf("Credentials saved (encrypted at rest, file mode 0600; machine-local key at %s). Store: %s. To source the key from a secrets manager instead, set api_key to a secret://env|file|cmd reference in agent-harness.yml.", config.MachineKeyPath(), config.SecureConfigPath()))
}

// refreshStoredModel updates the stored model for an unchanged stored
// provider key so a provider switch survives restarts without
// re-authenticating.
func (app *App) refreshStoredModel(provider, model string) {
	credManager := config.NewCredentialManager()
	if secureCfg, err := credManager.LoadSecure(); err == nil && secureCfg.Provider == provider {
		secureCfg.Model = model
		_ = credManager.SaveSecure(secureCfg)
	}
}
