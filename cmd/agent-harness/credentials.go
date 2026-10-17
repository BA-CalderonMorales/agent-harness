package main

import (
	"fmt"
	"os"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
)

// loadCredentials handles secure credential loading and migration.
//
// Boot must never die on credentials: a locked store, a wrong master
// password, or a missing key leaves the TUI reachable with the provider
// marked misconfigured and a pointer to /login, which reconfigures the
// store from inside the app.
func (app *App) loadCredentials(credManager *config.CredentialManager) error {
	if config.IsLocalProvider(app.config.Provider) {
		if app.config.APIKey == "" {
			app.config.APIKey = app.config.Provider
		}
		return nil
	}

	if app.config.APIKey != "" {
		return nil
	}

	if credManager.HasSecureCredentials() {
		// The store unlocks with the machine-local key (auto-generated,
		// 0600) — no console prompt, no exit path. Only a store from an
		// older password-based release fails here, and it degrades
		// gracefully to a /login pointer.
		secureCfg, err := credManager.LoadSecure()
		if err != nil {
			app.bootNotice = sprintf("Credential store (%s) could not be unlocked: %v. Run /login to reconfigure.", config.SecureConfigPath(), err)
			return nil
		}
		app.applySecureConfig(secureCfg)
	}

	if app.config.APIKey == "" && credManager.HasLegacyCredentials() {
		app.migrateLegacyCredentials(credManager)
	}

	if app.config.APIKey == "" {
		app.bootNotice = sprintf("No API key configured for provider %q. Run /login or set it in Settings (keys are stored encrypted at %s).", app.config.Provider, config.SecureConfigPath())
	}

	return nil
}

// storedKeyForProvider resolves the API key to use when the login
// wizard finishes without a typed key. Priority: an env-pinned key
// (explicit for the session, boot already landed it in config), then
// the encrypted store's key FOR THIS PROVIDER, then a config key that
// was already authenticating this provider. config.APIKey alone is not
// trustworthy here: for local providers it holds the dummy provider
// name, and after a provider switch it can hold a key minted for a
// different provider — both went out as Bearer garbage and 401'd
// despite the wizard saying "using stored API key". Local providers
// never reach this decision (they keep their dummy key by design).
func (app *App) storedKeyForProvider(provider string, credManager *config.CredentialManager) (string, bool) {
	if provider == "" || config.IsLocalProvider(provider) {
		return "", false
	}
	if envPinnedKey(provider) {
		return app.config.APIKey, app.config.APIKey != ""
	}
	if credManager.HasSecureCredentials() {
		if secureCfg, err := credManager.LoadSecure(); err == nil {
			if key, ok := secureCfg.ProviderKeys[provider]; ok && key != "" {
				return key, true
			}
			// Legacy single-key store: the stored key counts only for
			// the provider it was minted for.
			if secureCfg.Provider == provider && secureCfg.APIKey != "" {
				return secureCfg.APIKey, true
			}
		}
	}
	if app.config.APIKey != "" && app.config.Provider == provider {
		return app.config.APIKey, true
	}
	return "", false
}

// envPinnedKey reports whether an environment variable explicitly pins
// the key for this session. Generic vars apply to any provider;
// provider-specific vars only to their own.
func envPinnedKey(provider string) bool {
	if os.Getenv("AH_API_KEY") != "" || os.Getenv("AGENT_HARNESS_API_KEY") != "" {
		return true
	}
	switch provider {
	case "openrouter":
		return os.Getenv("OPENROUTER_API_KEY") != ""
	case "nvidia":
		return os.Getenv("NVIDIA_API_KEY") != ""
	case "omniroute":
		return os.Getenv("OMNITROUTE_API_KEY") != ""
	}
	return false
}

// applySecureConfig applies secure configuration values.
// Environment variables take precedence over saved credentials.
func (app *App) applySecureConfig(secureCfg *config.SecureConfig) {
	app.secureConfig = secureCfg

	envProviderPinned := os.Getenv("AH_PROVIDER") != "" || os.Getenv("AGENT_HARNESS_PROVIDER") != ""
	envKeyPinned := os.Getenv("AH_API_KEY") != "" || os.Getenv("AGENT_HARNESS_API_KEY") != ""

	// Layered settings own the provider, exactly as they own the model
	// below. The store's Provider is the last provider logged into — a
	// single slot, not a preference — so letting it win dragged a
	// machine back to `local` on every boot (and to the login wall
	// behind it) while a provider sat chosen on disk. It still fills
	// the gap for a machine that has never picked one.
	if secureCfg.Provider != "" && !envProviderPinned && !app.config.LayerSet("provider") {
		app.config.Provider = secureCfg.Provider
	}

	// The key must belong to the provider that actually won. The store's
	// active key is minted for the store's provider, so applying it
	// after a provider change hands one service another's secret —
	// local's dummy key travelling as a Bearer token was the "using
	// stored API key" 401. Prefer the key recorded for this provider,
	// keep a config/env key that was already authenticating it, and
	// take the single-slot value only when it was minted here.
	if !envKeyPinned && app.config.APIKey == "" {
		switch {
		case secureCfg.APIKey != "" && secureCfg.Provider == app.config.Provider:
			app.config.APIKey = secureCfg.APIKey
		default:
			if key := secureCfg.ProviderKeys[app.config.Provider]; key != "" {
				app.config.APIKey = key
			}
		}
	}
	// Layered settings own the preferred model; the credential store may
	// still contain the model chosen during an earlier login.
	if app.config.Model == "" && secureCfg.Model != "" && os.Getenv("AH_MODEL") == "" && os.Getenv("AGENT_HARNESS_MODEL") == "" {
		app.config.Model = secureCfg.Model
	}
}

// migrateLegacyCredentials migrates from legacy format.
func (app *App) migrateLegacyCredentials(credManager *config.CredentialManager) {
	fmt.Println("Found existing credentials in legacy format.")
	secureCfg, err := credManager.MigrateFromLegacy()
	if err != nil {
		fmt.Printf("Migration failed: %v\n", err)
	} else {
		app.applySecureConfig(secureCfg)
	}
}

// initSession initializes the session manager and creates or resumes a session.
