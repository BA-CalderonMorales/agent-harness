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
		if secureCfg, err := credManager.LoadSecure(); err == nil &&
			secureCfg.Provider == provider && secureCfg.APIKey != "" {
			return secureCfg.APIKey, true
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
	}
	return false
}

// applySecureConfig applies secure configuration values.
// Environment variables take precedence over saved credentials.
func (app *App) applySecureConfig(secureCfg *config.SecureConfig) {
	app.secureConfig = secureCfg
	if secureCfg.Provider != "" && os.Getenv("AH_PROVIDER") == "" && os.Getenv("AGENT_HARNESS_PROVIDER") == "" {
		app.config.Provider = secureCfg.Provider
	}
	if secureCfg.APIKey != "" && os.Getenv("AH_API_KEY") == "" && os.Getenv("AGENT_HARNESS_API_KEY") == "" {
		app.config.APIKey = secureCfg.APIKey
	}
	if secureCfg.Model != "" && os.Getenv("AH_MODEL") == "" && os.Getenv("AGENT_HARNESS_MODEL") == "" {
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
