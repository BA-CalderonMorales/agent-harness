package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/testharness/testenv"
)

// Upgrade compatibility for user settings: testdata/compat holds a
// settings.json for each generation of keys a release wrote (v0.3.5 added
// the local-runtime keys, v0.3.38 the tagline, v0.4.1 the theme and
// reasoning effort). Every build must read each one into the same choices
// and keep the user's other values when it saves a change.

type settingsGeneration struct {
	file          string
	provider      string
	model         string
	endpoint      string
	contextLength int
	temperature   float64
	permission    PermissionMode
	theme         string
	effort        string
	// runtime and persona are checked only where the fixture writes a
	// non-empty value; an empty or absent key makes no claim to check.
	runtime   string
	persona   string
	maxTokens int
	// Every generation writes all four permission switches, which makes
	// them an explicit choice that overrides the permission mode.
	permRead, permWrite, permDelete, permExecute bool
}

var settingsGenerations = []settingsGeneration{
	{file: "settings-v0.3.5.json", provider: "local", model: "ornith-q4", endpoint: "http://127.0.0.1:8080/v1",
		contextLength: 32768, temperature: 0.2, permission: PermissionWorkspaceWrite,
		runtime: "llamacpp", persona: "developer", maxTokens: 4096,
		permRead: true, permWrite: true, permExecute: true},
	{file: "settings-v0.3.38.json", provider: "openrouter", model: "nvidia/nemotron-3.5-lightning:free",
		endpoint: "https://openrouter.ai/api/v1", contextLength: 131072, temperature: 0.7, permission: PermissionWorkspaceWrite,
		persona: "developer", maxTokens: 8192,
		permRead: true, permWrite: true, permExecute: true},
	// v0.4.1 wrote no max_tokens, so the documented default applies.
	{file: "settings-v0.4.1.json", provider: "lemonade", model: "qwen3.5-4b-FLM", endpoint: "http://127.0.0.1:13305/v1",
		contextLength: 4096, temperature: 0, permission: PermissionDangerFullAccess, theme: "ember", effort: "medium",
		runtime: "lemonade", maxTokens: DefaultMaxTokens,
		permRead: true, permWrite: true, permDelete: true, permExecute: true},
}

func readSettingsFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "compat", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

func assertGeneration(t *testing.T, cfg *LayeredConfig, want settingsGeneration) {
	t.Helper()
	if cfg.Provider != want.provider || cfg.Model != want.model || cfg.EndpointURL != want.endpoint {
		t.Errorf("provider/model/endpoint = %q/%q/%q, want %q/%q/%q",
			cfg.Provider, cfg.Model, cfg.EndpointURL, want.provider, want.model, want.endpoint)
	}
	if cfg.ContextLength != want.contextLength || cfg.Temperature != want.temperature || !cfg.TemperatureSet {
		t.Errorf("context/temperature = %d/%v (set %v), want %d/%v (set)",
			cfg.ContextLength, cfg.Temperature, cfg.TemperatureSet, want.contextLength, want.temperature)
	}
	if cfg.PermissionMode != want.permission {
		t.Errorf("permission mode = %v, want %v", cfg.PermissionMode, want.permission)
	}
	if cfg.Theme != want.theme || cfg.Effort != want.effort {
		t.Errorf("theme/effort = %q/%q, want %q/%q", cfg.Theme, cfg.Effort, want.theme, want.effort)
	}
	if want.runtime != "" && cfg.Runtime != want.runtime {
		t.Errorf("runtime = %q, want %q", cfg.Runtime, want.runtime)
	}
	if want.persona != "" && cfg.Persona != want.persona {
		t.Errorf("persona = %q, want %q", cfg.Persona, want.persona)
	}
	if cfg.MaxTokens != want.maxTokens {
		t.Errorf("max tokens = %d, want %d", cfg.MaxTokens, want.maxTokens)
	}
	got := [4]bool{cfg.PermRead, cfg.PermWrite, cfg.PermDelete, cfg.PermExecute}
	if wantPerms := [4]bool{want.permRead, want.permWrite, want.permDelete, want.permExecute}; got != wantPerms || !cfg.PermExplicit {
		t.Errorf("read/write/delete/execute = %v (explicit %v), want %v (explicit)", got, cfg.PermExplicit, wantPerms)
	}
}

func TestSettingsFromEveryGenerationLoad(t *testing.T) {
	for _, generation := range settingsGenerations {
		t.Run(generation.file, func(t *testing.T) {
			configHome := t.TempDir()
			t.Setenv("AGENT_HARNESS_CONFIG_HOME", configHome)
			clearConfigEnv(t)
			if err := os.WriteFile(filepath.Join(configHome, "settings.json"), readSettingsFixture(t, generation.file), 0o600); err != nil {
				t.Fatal(err)
			}

			cfg, err := NewLayeredLoader(t.TempDir()).Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			assertGeneration(t, cfg, generation)
		})
	}
}

// Changing one setting after an upgrade rewrites settings.json. Every
// other value the earlier build wrote must survive, keys this build no
// longer reads included.
func TestSavingAChangeKeepsEveryGenerationsOtherValues(t *testing.T) {
	for _, generation := range settingsGenerations {
		t.Run(generation.file, func(t *testing.T) {
			configHome := t.TempDir()
			t.Setenv("AGENT_HARNESS_CONFIG_HOME", configHome)
			clearConfigEnv(t)
			path := filepath.Join(configHome, "settings.json")
			// A key no build reads stands in for a setting a later release
			// retires: an upgraded user's file may still hold it.
			var withRetired map[string]interface{}
			if err := json.Unmarshal(readSettingsFixture(t, generation.file), &withRetired); err != nil {
				t.Fatal(err)
			}
			withRetired["retired_setting"] = "kept-across-upgrades"
			original, err := json.Marshal(withRetired)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatal(err)
			}

			if err := NewLayeredLoader(t.TempDir()).SaveSettings(SourceUser, map[string]interface{}{"model": "changed-after-upgrade"}); err != nil {
				t.Fatalf("SaveSettings: %v", err)
			}

			var before, after map[string]interface{}
			if err := json.Unmarshal(original, &before); err != nil {
				t.Fatal(err)
			}
			saved, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(saved, &after); err != nil {
				t.Fatalf("saved settings are not JSON: %v", err)
			}
			for key, value := range before {
				if key == "model" {
					continue
				}
				if got, ok := after[key]; !ok || got != value {
					t.Errorf("key %q = %v (present %v), want %v kept", key, got, ok, value)
				}
			}

			cfg, err := NewLayeredLoader(t.TempDir()).Load()
			if err != nil {
				t.Fatalf("Load after save: %v", err)
			}
			want := generation
			want.model = "changed-after-upgrade"
			assertGeneration(t, cfg, want)
		})
	}
}

// v0.3.x kept settings.json in ~/.agent-harness; the first boot of a newer
// build moves it to the XDG config home, and the user's choices must load
// from there unchanged.
func TestLegacyHomeSettingsLoadAfterMigration(t *testing.T) {
	generation := settingsGenerations[0]
	home := t.TempDir()
	testenv.SetHome(t, home)
	clearConfigEnv(t)
	legacy := filepath.Join(home, ".agent-harness")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "settings.json"), readSettingsFixture(t, generation.file), 0o600); err != nil {
		t.Fatal(err)
	}

	MigrateLegacyHome()

	if _, err := os.Stat(filepath.Join(ConfigHome(), "settings.json")); err != nil {
		t.Fatalf("settings.json was not moved to the XDG config home: %v", err)
	}
	cfg, err := NewLayeredLoader(t.TempDir()).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	assertGeneration(t, cfg, generation)
}
