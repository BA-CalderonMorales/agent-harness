package main

import (
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
)

// userSettingsValues builds the delta persistUserSettings writes to the
// user layer, and the names of the guarded values it refused to include.
// It is pure: no I/O, no notices.
//
// The user layer is a delta over the tracked project config, so a value
// that carries no information must not be written. Persisting a zero or
// empty froze it as an override: context_length 0 clobbered the project's
// context window, and an empty endpoint/model clobbered the project's
// provider wiring.
//
// The provider carries the same rule and it is the costly one to miss:
// an empty string is dropped on read, so a blank write erased the user's
// provider and sent the next boot to the default - `local` and the login
// wall the dead local probe raises behind it. It could not even be
// repaired from the app, because the blank write repeated.
//
// An environment pin is a per-session override, not a preference:
// provider, model and endpoint it pinned stay out of the user layer so
// they do not outlive the environment variable. A pinned value is skipped,
// not rejected.
//
// Blank runtime, effort and theme are skipped silently: unset is their
// normal state, so they are not worth a notice.
func userSettingsValues(cfg *config.LayeredConfig) (map[string]interface{}, []string) {
	values := map[string]interface{}{
		"temperature":     cfg.Temperature, // 0.0 is a valid choice
		"permission_mode": cfg.PermissionMode.String(),
		"perm_read":       cfg.PermRead,
		"perm_write":      cfg.PermWrite,
		"perm_delete":     cfg.PermDelete,
		"perm_execute":    cfg.PermExecute,
		"tagline":         cfg.Tagline, // blank is meaningful: hide the tagline
	}
	var rejected []string
	text := func(key, value string, pinned bool) {
		switch {
		case pinned:
		case value == "":
			rejected = append(rejected, key)
		default:
			values[key] = value
		}
	}
	number := func(key string, value int) {
		if value > 0 {
			values[key] = value
		} else {
			rejected = append(rejected, key)
		}
	}
	// A pinned provider also pins what boot derived from it (its default
	// endpoint, model, context and token budget): persisted beside the
	// user's own provider they form a mismatched pair.
	derived := cfg.ProviderPinned
	text("provider", cfg.Provider, cfg.ProviderPinned)
	text("endpoint_url", cfg.EndpointURL, cfg.EndpointPinned || derived)
	text("model", cfg.Model, cfg.ModelPinned || derived)
	if !derived {
		number("context_length", cfg.ContextLength)
		number("max_tokens", cfg.MaxTokens)
	}
	if cfg.Runtime != "" && !cfg.RuntimePinned {
		values["runtime"] = cfg.Runtime
	}
	if cfg.Effort != "" {
		values["reasoning_effort"] = cfg.Effort
	}
	if cfg.Theme != "" {
		values["theme"] = cfg.Theme
	}
	// Labs follow the same pin rule: an env-pinned flag is left out.
	for key, value := range cfg.LabSettingsValues() {
		values[key] = value
	}
	return values, rejected
}

// persistUserSettings writes the user's runtime preferences to the user
// config layer (~/.agent-harness/settings.json). API keys stay out: they
// belong to the encrypted credential store. A guarded value that is blank
// or non-positive is left out of the write and reported as a warning; the
// write itself touches no network.
func (app *App) persistUserSettings() {
	values, rejected := userSettingsValues(app.config)
	if len(rejected) > 0 {
		app.warnSettings(sprintf("Warning: not saving blank settings: %s", strings.Join(rejected, ", ")))
	}
	loader := config.NewLayeredLoader(app.cwd)
	if err := loader.SaveSettings(config.SourceUser, values); err != nil {
		app.warnSettings(sprintf("Warning: failed to save settings: %v", err))
	}
	// A lab settings.local.json sets is saved there: it outranks the
	// user layer, so a toggle written anywhere else would not last.
	if local := app.config.LabLocalValues(); len(local) > 0 {
		if err := loader.SaveSettings(config.SourceLocal, local); err != nil {
			app.warnSettings(sprintf("Warning: failed to save settings.local.json: %v", err))
		}
	}
}

// warnSettings reports a settings-write problem through the status line.
func (app *App) warnSettings(msg string) {
	if app.tuiApp != nil {
		app.tuiApp.Send(tui.StatusMsg{Text: msg, Type: "warning"})
	}
}

// commitConfigChange persists the current runtime configuration after any
// in-session mutation so provider/model choices survive restarts.
func (app *App) commitConfigChange() {
	app.persistUserSettings()
}

// storedModelForProvider reads the last model used with a provider
// from the encrypted store. Empty when the store holds none.
func (app *App) storedModelForProvider(provider string) string {
	credManager := config.NewCredentialManager()
	if secureCfg, err := credManager.LoadSecure(); err == nil && secureCfg != nil {
		if m, ok := secureCfg.ProviderModels[provider]; ok {
			return m
		}
	}
	return ""
}

// storeProviderModel persists the last model used with a provider —
// the retention half of the per-provider credential set: logging back
// into this provider pre-selects this model.
func (app *App) storeProviderModel(provider, model string) {
	if provider == "" || model == "" {
		return
	}
	credManager := config.NewCredentialManager()
	existing, err := credManager.LoadSecure()
	if err != nil || existing == nil {
		existing = &config.SecureConfig{Provider: provider, APIKey: provider}
		if config.IsLocalProvider(provider) {
			existing.APIKey = provider
		}
	}
	if existing.ProviderModels == nil {
		existing.ProviderModels = map[string]string{}
	}
	existing.ProviderModels[provider] = model
	existing.Provider = provider
	existing.Model = model
	_ = credManager.SaveSecure(existing)
}

// saveTypedCredentials stores a freshly typed key in the encrypted
// credential store, MERGING into the per-provider key set: saving the
// nvidia key must not clobber the openrouter key (the single-slot
// store used to lose them on every provider switch). The narration is
// returned, not sent.
func (app *App) saveTypedCredentials(provider, apiKey, model string) string {
	credManager := config.NewCredentialManager()
	keys := map[string]string{}
	models := map[string]string{}
	var existing *config.SecureConfig
	if cfg, err := credManager.LoadSecure(); err == nil && cfg != nil {
		existing = cfg
		for p, k := range existing.ProviderKeys {
			keys[p] = k
		}
		for p, m := range existing.ProviderModels {
			models[p] = m
		}
	}
	keys[provider] = apiKey
	models[provider] = model

	secureCfg := &config.SecureConfig{
		Provider:       provider,
		APIKey:         apiKey,
		Model:          model,
		ProviderKeys:   keys,
		ProviderModels: models,
	}
	if err := credManager.SaveSecure(secureCfg); err != nil {
		return sprintf("[!] Failed to save credentials: %v", err)
	}
	return sprintf("Credentials saved (encrypted at rest, file mode 0600; machine-local key at %s). Store: %s. To source the key from a secrets manager instead, set api_key to a secret://env|file|cmd reference in agent-harness.yml.", config.MachineKeyPath(), config.SecureConfigPath())
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
