package main

import (
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"os"
	"path/filepath"
)

// requireGitRepo returns an error if the app is not inside a git repository.
func (app *App) requireGitRepo() error {
	if app.gitContext == nil || !app.gitContext.IsRepo {
		return fmt.Errorf("not in a git repository")
	}
	return nil
}

// logout clears credentials from memory and secure storage.
func (app *App) logout() error {
	app.config.APIKey = ""
	app.secureConfig = nil
	credManager := config.NewCredentialManager()
	if err := credManager.ClearSecureConfig(); err != nil {
		return errf("failed to clear credentials: %w", err)
	}
	return nil
}

// startLogin opens the modal login wizard. The dialog captures the
// provider, a masked API key (typed or pasted), and the model; the
// completion handler persists the key to the encrypted store. Stored
// per-provider keys are retained: the dialog shows a masked hint and
// finishes without re-entry when the store already holds a key for the
// chosen provider.
func (app *App) startLogin() error {
	if app.tuiApp == nil {
		return errf("login wizard only available in TUI mode")
	}
	app.tuiApp.OpenLoginDialog(app.storedCredentialsSnapshot())
	return nil
}

// storedCredentialsSnapshot reads the encrypted store's per-provider
// key set for the login wizard. The fallback covers a config/env key
// that was authenticating the current provider but never saved to the
// store.
func (app *App) storedCredentialsSnapshot() tui.StoredCredentials {
	keys := map[string]string{}
	primary := ""
	credManager := config.NewCredentialManager()
	if secureCfg, err := credManager.LoadSecure(); err == nil && secureCfg != nil {
		for provider, key := range secureCfg.ProviderKeys {
			keys[provider] = key
		}
		primary = secureCfg.APIKey
	} else if app.config.APIKey != "" && !config.IsLocalProvider(app.config.APIKey) {
		primary = app.config.APIKey
	}
	return tui.NewStoredCredentials(keys, primary)
}

// startProviderPicker opens the provider-switch modal. Picking a
// provider opens the model picker with that provider's full model list;
// the stored key is retained, so switching never requires re-auth.
func (app *App) startProviderPicker() error {
	if app.tuiApp == nil {
		return errf("provider picker only available in TUI mode")
	}
	app.tuiApp.OpenProviderPicker()
	return nil
}

// pickProvider applies a provider picked in the provider modal, then
// opens the model picker with the full model list for that provider.
// Picking the already-active provider still opens the picker — the
// modal's job is switching models, not just providers.
func (app *App) pickProvider(provider string, tuiApp *tui.App) {
	if provider == "" {
		return
	}
	if provider != app.config.Provider {
		app.config.Provider = provider
		app.config.EndpointURL = app.wizardEndpoint(provider)
		if config.IsLocalProvider(provider) {
			app.config.APIKey = provider
		}
		if app.config.Model == "" {
			app.config.Model = getDefaultModel(provider)
		}
		app.config.ApplyTimeoutDefaults()
		app.commitConfigChange()
		app.rebuildLLMClient()
	}
	tuiApp.ShowModelPicker(app.modelsForProvider(provider), provider)
}

// startModelPicker opens the model picker for the active provider with
// its full model list — the fast way to switch models when a provider
// and key are already configured.
func (app *App) startModelPicker() error {
	if app.tuiApp == nil {
		return errf("model picker only available in TUI mode")
	}
	provider := app.config.Provider
	if provider == "" {
		provider = config.DefaultProvider
	}
	app.tuiApp.ShowModelPicker(app.modelsForProvider(provider), provider)
	return nil
}

// modelsForProvider returns the full model list for the current client,
// falling back to the static catalog when the provider has no dynamic
// listing (local/ollama) or the API call fails. Live entries carry the
// provider-advertised context length; models the endpoint does not
// describe inherit the catalog value so the budget bar stays honest.
func (app *App) modelsForProvider(provider string) []tui.ModelItem {
	if c, ok := app.client.(*llm.HTTPClient); ok && c != nil {
		if infos, err := c.ListModelsDetailed(); err == nil && len(infos) > 0 {
			catalog := getModelsForProvider(provider, app.session.Model)
			catalogCtx := make(map[string]int, len(catalog))
			for _, item := range catalog {
				catalogCtx[item.ID] = item.ContextLen
			}
			items := make([]tui.ModelItem, 0, len(infos))
			for _, info := range infos {
				item := tui.ModelItem{ID: info.ID, Name: info.ID, Provider: provider}
				if info.ContextLength > 0 {
					item.ContextLen = info.ContextLength
				} else {
					item.ContextLen = catalogCtx[info.ID]
				}
				items = append(items, item)
			}
			return items
		}
	}
	return getModelsForProvider(provider, app.session.Model)
}

// reset clears all credentials and sessions.
func (app *App) reset() error {
	credManager := config.NewCredentialManager()
	if err := credManager.ClearSecureConfig(); err != nil {
		return errf("failed to clear credentials: %w", err)
	}
	sessions, err := app.sessionManager.ListSessions()
	if err != nil {
		return errf("failed to list sessions: %w", err)
	}
	for _, s := range sessions {
		path := filepath.Join(app.getSessionsDir(), s.ID+".json")
		_ = os.Remove(path)
	}
	app.session = app.session.Clear()
	return nil
}

// formatSessionList formats sessions for display.
