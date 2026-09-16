package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
)

// newCredentialTestEnv redirects the credential store to a temp home
// and seeds it with a secure config. Credential paths derive from
// os.UserHomeDir, so HOME is the redirect point.
func newCredentialTestEnv(t *testing.T) *config.CredentialManager {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AH_API_KEY", "")
	t.Setenv("AGENT_HARNESS_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("NVIDIA_API_KEY", "")
	return config.NewCredentialManager()
}

// THE regression: boot with a local provider sets config.APIKey to the
// dummy provider name and never consults the store; /login to a hosted
// provider then sent "Bearer local" and 401'd despite "using stored
// API key". The store's key for the target provider must win.
func TestStoredKeyForProvider_PrefersStoreOverLocalDummy(t *testing.T) {
	cm := newCredentialTestEnv(t)
	if err := cm.SaveSecure(&config.SecureConfig{Provider: "openrouter", APIKey: "sk-or-real", Model: "m"}); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	app := &App{config: &config.LayeredConfig{Provider: "local", APIKey: "local"}}
	key, ok := app.storedKeyForProvider("openrouter", cm)
	if !ok || key != "sk-or-real" {
		t.Fatalf("storedKeyForProvider = (%q, %v), want the store key for openrouter", key, ok)
	}
}

// Cross-provider leftovers must not authenticate: a store key minted
// for another provider is an honest "no key for this provider".
func TestStoredKeyForProvider_RejectsForeignStoreKey(t *testing.T) {
	cm := newCredentialTestEnv(t)
	if err := cm.SaveSecure(&config.SecureConfig{Provider: "nvidia", APIKey: "nvapi-x"}); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	app := &App{config: &config.LayeredConfig{Provider: "nvidia", APIKey: "nvapi-x"}}
	if _, ok := app.storedKeyForProvider("openrouter", cm); ok {
		t.Fatal("a nvidia store key must not authenticate openrouter")
	}
}

// Re-login to the same hosted provider without the store: the config
// key was already authenticating this provider and stays valid.
func TestStoredKeyForProvider_KeepsConfigKeyOnSameProvider(t *testing.T) {
	cm := newCredentialTestEnv(t)
	app := &App{config: &config.LayeredConfig{Provider: "openai", APIKey: "sk-config"}}
	key, ok := app.storedKeyForProvider("openai", cm)
	if !ok || key != "sk-config" {
		t.Fatalf("storedKeyForProvider = (%q, %v), want the config key", key, ok)
	}
}

// An env-pinned key is explicit for the session and outranks the store.
func TestStoredKeyForProvider_EnvPinWins(t *testing.T) {
	cm := newCredentialTestEnv(t)
	t.Setenv("AH_API_KEY", "sk-env")
	if err := cm.SaveSecure(&config.SecureConfig{Provider: "openai", APIKey: "sk-store"}); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	app := &App{config: &config.LayeredConfig{Provider: "openai", APIKey: "sk-env"}}
	key, ok := app.storedKeyForProvider("openai", cm)
	if !ok || key != "sk-env" {
		t.Fatalf("storedKeyForProvider = (%q, %v), want the env-pinned key", key, ok)
	}

	// Provider-specific env vars only cover their own provider. NVIDIA
	// has no store entry, so only the env var can supply a key.
	t.Setenv("AH_API_KEY", "")
	app2 := &App{config: &config.LayeredConfig{Provider: "nvidia", APIKey: ""}}
	if _, ok := app2.storedKeyForProvider("nvidia", cm); ok {
		t.Fatal("no store entry and no env var must resolve to no key")
	}
	t.Setenv("NVIDIA_API_KEY", "sk-nv-env")
	app3 := &App{config: &config.LayeredConfig{Provider: "nvidia", APIKey: "sk-nv-env"}}
	key, ok = app3.storedKeyForProvider("nvidia", cm)
	if !ok || key != "sk-nv-env" {
		t.Fatalf("NVIDIA_API_KEY must pin nvidia, got (%q, %v)", key, ok)
	}
}

// The local exception: local providers never resolve through the store
// path — they keep their dummy key by design.
func TestStoredKeyForProvider_LocalException(t *testing.T) {
	cm := newCredentialTestEnv(t)
	if err := cm.SaveSecure(&config.SecureConfig{Provider: "openrouter", APIKey: "sk-or-real"}); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	app := &App{config: &config.LayeredConfig{Provider: "local", APIKey: "local"}}
	for _, provider := range []string{"local", "ollama"} {
		if _, ok := app.storedKeyForProvider(provider, cm); ok {
			t.Fatalf("local provider %q must not resolve through the store path", provider)
		}
	}
}

// The wizard hint must never render for the local dummy — a masked
// hint built on "local" promised a stored key that did not exist.
func TestLoginKeyHint_HidesLocalDummy(t *testing.T) {
	newCredentialTestEnv(t)

	app := &App{config: &config.LayeredConfig{Provider: "local", APIKey: "local"}}
	if hint := app.storedCredentialsSnapshot(); hint.Primary() != "" {
		t.Fatalf("local dummy rendered a stored-key hint %q", hint.Primary())
	}

	app = &App{config: &config.LayeredConfig{Provider: "openai", APIKey: "sk-config"}}
	if hint := app.storedCredentialsSnapshot(); !strings.Contains(hint.Primary(), "sk") && !strings.Contains(hint.Primary(), "…") {
		t.Fatalf("hosted config key hint = %q, want a masked hint", hint.Primary())
	}
}

// THE regression: a provider on disk must survive a boot whose store
// still names the last provider logged into. The store's Provider is a
// single slot, and letting it win sent a machine that had chosen a
// hosted provider back to `local` on every launch — and to the login
// wall the unreachable local probe raises behind it.
func TestBootKeepsProviderChosenOnDisk(t *testing.T) {
	cm := newCredentialTestEnv(t)
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", t.TempDir())
	t.Setenv("AH_PROVIDER", "")
	t.Setenv("AGENT_HARNESS_PROVIDER", "")

	// The store's active slot belongs to a provider the user has since
	// moved off; the key for the chosen provider is recorded beside it.
	if err := cm.SaveSecure(&config.SecureConfig{
		Provider:     "local",
		APIKey:       "local",
		ProviderKeys: map[string]string{"local": "local", "nvidia": "nvapi-chosen"},
	}); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	writeUserSettings(t, map[string]string{
		"provider":     "nvidia",
		"model":        "deepseek-ai/deepseek-v4-flash-0731",
		"endpoint_url": "https://integrate.api.nvidia.com/v1",
	})

	cfg, err := config.NewLayeredLoader(t.TempDir()).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	app := &App{config: cfg}
	app.applySecureConfig(mustLoadSecure(t, cm))

	if app.config.Provider != "nvidia" {
		t.Fatalf("provider = %q, want nvidia: the store's slot must not override a provider on disk", app.config.Provider)
	}
	// The key must follow the provider that won — a local dummy token
	// travelling as a Bearer credential was the 401 this replaces.
	if app.config.APIKey != "nvapi-chosen" {
		t.Fatalf("api key = %q, want the key recorded for nvidia", app.config.APIKey)
	}
	if app.config.Model != "deepseek-ai/deepseek-v4-flash-0731" {
		t.Fatalf("model = %q, want the model chosen on disk", app.config.Model)
	}
}

// The store still fills the gap for a machine that has never picked a
// provider: no layer names one, so the last login stands.
func TestBootFallsBackToStoredProviderWhenNoLayerNamesOne(t *testing.T) {
	cm := newCredentialTestEnv(t)
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", t.TempDir())
	t.Setenv("AH_PROVIDER", "")
	t.Setenv("AGENT_HARNESS_PROVIDER", "")

	if err := cm.SaveSecure(&config.SecureConfig{Provider: "openrouter", APIKey: "sk-or-real"}); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	cfg, err := config.NewLayeredLoader(t.TempDir()).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	app := &App{config: cfg}
	app.applySecureConfig(mustLoadSecure(t, cm))

	if app.config.Provider != "openrouter" {
		t.Fatalf("provider = %q, want the stored provider", app.config.Provider)
	}
	if app.config.APIKey != "sk-or-real" {
		t.Fatalf("api key = %q, want the stored key", app.config.APIKey)
	}
}

// writeUserSettings drops a user-layer settings.json into the config
// home the loader resolves from.
func writeUserSettings(t *testing.T, values map[string]string) {
	t.Helper()
	path := filepath.Join(config.ConfigHome(), "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll(%s): %v", path, err)
	}
	data, err := json.Marshal(values)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

// mustLoadSecure reads the seeded store, which applySecureConfig takes
// as a value rather than a manager.
func mustLoadSecure(t *testing.T, cm *config.CredentialManager) *config.SecureConfig {
	t.Helper()
	cfg, err := cm.LoadSecure()
	if err != nil {
		t.Fatalf("LoadSecure() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadSecure() returned a nil config")
	}
	return cfg
}
