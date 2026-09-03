package main

import (
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
	if hint := app.loginKeyHint(); hint != "" {
		t.Fatalf("local dummy rendered a stored-key hint %q", hint)
	}

	app = &App{config: &config.LayeredConfig{Provider: "openai", APIKey: "sk-config"}}
	if hint := app.loginKeyHint(); !strings.Contains(hint, "sk") && !strings.Contains(hint, "…") {
		t.Fatalf("hosted config key hint = %q, want a masked hint", hint)
	}
}
