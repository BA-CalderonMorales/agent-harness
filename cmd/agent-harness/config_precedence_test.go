package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
)

// The resolution contract, executable.
//
// Two kinds of knowledge meet at boot and must never compete, because
// they answer different questions:
//
//	secrets — "how do I authenticate to provider X?"  encrypted store,
//	          keyed BY provider
//	wiring  — "which provider/model does this workspace use?"  config
//	          layers, which CHOOSE the provider
//
// The store also carries a single "last provider logged into" slot.
// That is a fallback selection, not a preference, so it may fill a gap
// but must never outrank a provider on disk. Breaking either half of
// that is how a machine ends up pinned to the default provider with its
// own choice sitting on disk, which is what this table exists to catch.
func TestBootResolutionOrder(t *testing.T) {
	tests := []struct {
		name         string
		projectYML   string
		userSettings map[string]string
		store        *config.SecureConfig
		env          map[string]string
		wantProvider string
		wantAPIKey   string
	}{
		{
			name:       "a tracked project default does not outrank the user's choice",
			projectYML: "provider: local\nmodel: ornith-q4\n",
			userSettings: map[string]string{
				"provider": "nvidia",
				"model":    "deepseek-ai/deepseek-v4-flash-0731",
			},
			store: &config.SecureConfig{
				Provider:     "local",
				APIKey:       "local",
				ProviderKeys: map[string]string{"local": "local", "nvidia": "nvapi-chosen"},
			},
			wantProvider: "nvidia",
			wantAPIKey:   "nvapi-chosen",
		},
		{
			name:         "the store's slot is a fallback, not an override",
			projectYML:   "provider: local\nmodel: ornith-q4\n",
			userSettings: nil,
			store: &config.SecureConfig{
				Provider: "openrouter",
				APIKey:   "sk-or-real",
			},
			// No layer names a provider, so the project default stands and
			// the local dummy key is the honest answer for a local provider.
			wantProvider: "local",
			wantAPIKey:   "local",
		},
		{
			name:         "an environment pin outranks every file and the store",
			projectYML:   "provider: local\nmodel: ornith-q4\n",
			userSettings: map[string]string{"provider": "nvidia"},
			store: &config.SecureConfig{
				Provider: "local",
				APIKey:   "local",
			},
			env:          map[string]string{"AH_PROVIDER": "openrouter", "AH_API_KEY": "sk-env"},
			wantProvider: "openrouter",
			wantAPIKey:   "sk-env",
		},
		{
			name:         "a provider on disk with no key of its own is not handed another's",
			projectYML:   "",
			userSettings: map[string]string{"provider": "nvidia"},
			store: &config.SecureConfig{
				Provider:     "openrouter",
				APIKey:       "sk-or-real",
				ProviderKeys: map[string]string{"openrouter": "sk-or-real"},
			},
			wantProvider: "nvidia",
			// The store holds an openrouter key; using it for nvidia would
			// authenticate one service with another's secret. An empty key
			// is the honest answer — boot surfaces it as a login prompt.
			wantAPIKey: "",
		},
		{
			name:         "a blank provider in a layer never resolves to a blank provider",
			projectYML:   "",
			userSettings: map[string]string{"provider": "", "runtime": "", "reasoning_effort": ""},
			store:        nil,
			// An empty string carries no information, so it is dropped on
			// read rather than blanking the provider. Falling to the default
			// is intended here; being handed a blank is not.
			wantProvider: "local",
			wantAPIKey:   "local",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cm := newCredentialTestEnv(t)
			configHome := t.TempDir()
			t.Setenv("AGENT_HARNESS_CONFIG_HOME", configHome)
			cwd := t.TempDir()
			for _, key := range []string{
				"AH_PROVIDER", "AGENT_HARNESS_PROVIDER",
				"AH_API_KEY", "AGENT_HARNESS_API_KEY",
				"NVIDIA_API_KEY", "OPENROUTER_API_KEY",
			} {
				t.Setenv(key, "")
			}
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			if tc.projectYML != "" {
				if err := os.WriteFile(filepath.Join(cwd, "agent-harness.yml"), []byte(tc.projectYML), 0o644); err != nil {
					t.Fatalf("write project yml: %v", err)
				}
			}
			if tc.userSettings != nil {
				writeUserSettingsAt(t, configHome, tc.userSettings)
			}
			if tc.store != nil {
				if err := cm.SaveSecure(tc.store); err != nil {
					t.Fatalf("seed store: %v", err)
				}
			}

			cfg, err := config.NewLayeredLoader(cwd).Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			// The real boot path, not a stand-in: loadCredentials owns the
			// early returns (local dummy key, an already-configured key)
			// that decide whether the store is consulted at all.
			app := &App{config: cfg}
			if err := app.loadCredentials(cm); err != nil {
				t.Fatalf("loadCredentials() error = %v", err)
			}

			if app.config.Provider != tc.wantProvider {
				t.Errorf("provider = %q, want %q", app.config.Provider, tc.wantProvider)
			}
			if app.config.APIKey != tc.wantAPIKey {
				t.Errorf("api key = %q, want %q", app.config.APIKey, tc.wantAPIKey)
			}
			// A provider without a key must say so at boot rather than
			// dead-end later; the notice is the login handle.
			if tc.wantAPIKey == "" && app.bootNotice == "" {
				t.Error("no key resolved and no boot notice raised")
			}
		})
	}
}

// TestDefaultedLocalShortCircuitsTheStore pins a known gap rather than a
// contract, so a future fix has to change this test on purpose.
//
// loadCredentials returns early for a local provider — a local runtime
// authenticates with nothing, so it takes its dummy key and never asks
// the store. But Load() bakes DefaultProvider ("local") in when no layer
// names a provider, so "no layer chose one" is indistinguishable from
// "a layer chose local". The store's remembered provider is therefore
// never consulted on a machine with no provider in any layer: it boots
// local while a perfectly good key for another provider sits in the
// store.
//
// This is the residue of the same explicit-vs-defaulted confusion the
// provider precedence fix addressed. Closing it means consulting the
// store when the provider is only the built-in default — gating the
// short circuit on LayeredConfig.LayerSet("provider") instead of on the
// resolved value — and then still handing a genuinely local provider
// its dummy key afterwards.
func TestDefaultedLocalShortCircuitsTheStore(t *testing.T) {
	cm := newCredentialTestEnv(t)
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", t.TempDir())
	for _, key := range []string{"AH_PROVIDER", "AGENT_HARNESS_PROVIDER", "AH_API_KEY", "AGENT_HARNESS_API_KEY"} {
		t.Setenv(key, "")
	}
	if err := cm.SaveSecure(&config.SecureConfig{Provider: "openrouter", APIKey: "sk-or-real"}); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	cfg, err := config.NewLayeredLoader(t.TempDir()).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.LayerSet("provider") {
		t.Fatal("precondition: no layer should name a provider here")
	}

	app := &App{config: cfg}
	if err := app.loadCredentials(cm); err != nil {
		t.Fatalf("loadCredentials() error = %v", err)
	}

	if app.config.Provider != "local" {
		t.Fatalf("provider = %q: the gap has been closed, so this test should now assert the stored provider %q", app.config.Provider, "openrouter")
	}
	if app.config.APIKey != "local" {
		t.Fatalf("api key = %q, want the local dummy key", app.config.APIKey)
	}
}

// writeUserSettingsAt drops a user-layer settings.json into a config home.
func writeUserSettingsAt(t *testing.T, configHome string, values map[string]string) {
	t.Helper()
	data, err := json.Marshal(values)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configHome, "settings.json"), data, 0o600); err != nil {
		t.Fatalf("write settings: %v", err)
	}
}
