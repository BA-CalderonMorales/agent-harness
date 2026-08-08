package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeProjectYAML writes a minimal tracked agent-harness.yml into root.
func writeProjectYAML(t *testing.T, root string) {
	t.Helper()
	data := []byte("provider: local\nmodel: ornith-q4\nendpoint_url: http://127.0.0.1:8080/v1\n")
	if err := os.WriteFile(filepath.Join(root, "agent-harness.yml"), data, 0644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
}

func TestSaveSettings_RoundTrip(t *testing.T) {
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", t.TempDir())
	clearConfigEnv(t)

	ll := NewLayeredLoader(t.TempDir())
	values := map[string]interface{}{
		"provider":       "openrouter",
		"model":          "openai/gpt-4o",
		"endpoint_url":   "https://openrouter.ai/api/v1",
		"max_tokens":     2048,
		"temperature":    0.1,
		"context_length": 16384,
	}
	if err := ll.SaveSettings(SourceUser, values); err != nil {
		t.Fatalf("SaveSettings() error = %v", err)
	}

	cfg, err := NewLayeredLoader(t.TempDir()).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Provider != "openrouter" {
		t.Errorf("provider = %q, want openrouter", cfg.Provider)
	}
	if cfg.Model != "openai/gpt-4o" {
		t.Errorf("model = %q, want openai/gpt-4o", cfg.Model)
	}
	if cfg.EndpointURL != "https://openrouter.ai/api/v1" {
		t.Errorf("endpoint = %q", cfg.EndpointURL)
	}
	if cfg.MaxTokens != 2048 || cfg.Temperature != 0.1 || cfg.ContextLength != 16384 {
		t.Errorf("generation config = max:%d temp:%v ctx:%d", cfg.MaxTokens, cfg.Temperature, cfg.ContextLength)
	}
}

func TestUserSettingsOverrideProjectYAML(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", configHome)
	clearConfigEnv(t)

	root := t.TempDir()
	writeProjectYAML(t, root)

	ll := NewLayeredLoader(root)
	values := map[string]interface{}{
		"provider":     "openrouter",
		"model":        "openai/gpt-4o",
		"endpoint_url": "https://openrouter.ai/api/v1",
	}
	if err := ll.SaveSettings(SourceUser, values); err != nil {
		t.Fatalf("SaveSettings() error = %v", err)
	}

	cfg, err := NewLayeredLoader(root).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Provider != "openrouter" {
		t.Errorf("provider = %q, want openrouter (user layer must beat tracked project yml)", cfg.Provider)
	}
	if cfg.Model != "openai/gpt-4o" {
		t.Errorf("model = %q, want openai/gpt-4o", cfg.Model)
	}
	if cfg.EndpointURL != "https://openrouter.ai/api/v1" {
		t.Errorf("endpoint = %q", cfg.EndpointURL)
	}
}

func TestEnvProviderMovesEndpointToProviderDefault(t *testing.T) {
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", t.TempDir())
	clearConfigEnv(t)
	t.Setenv("AH_API_KEY", "sk-test-123")
	t.Setenv("AH_PROVIDER", "openrouter")

	root := t.TempDir()
	// A tracked project yml pins a local llama.cpp endpoint; the env-driven
	// provider switch must not keep sending provider traffic there.
	writeProjectYAML(t, root)

	cfg, err := NewLayeredLoader(root).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Provider != "openrouter" {
		t.Fatalf("provider = %q, want openrouter", cfg.Provider)
	}
	if cfg.EndpointURL != "https://openrouter.ai/api/v1" {
		t.Errorf("endpoint = %q, want openrouter default (must follow env provider)", cfg.EndpointURL)
	}
	if cfg.APIKey != "sk-test-123" {
		t.Errorf("api key = %q, want sk-test-123", cfg.APIKey)
	}
}

func TestEnvEndpointStillWins(t *testing.T) {
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", t.TempDir())
	clearConfigEnv(t)
	t.Setenv("AH_PROVIDER", "openrouter")
	t.Setenv("AH_ENDPOINT_URL", "https://proxy.example.com/v1")

	cfg, err := NewLayeredLoader(t.TempDir()).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.EndpointURL != "https://proxy.example.com/v1" {
		t.Errorf("endpoint = %q, want explicit env endpoint", cfg.EndpointURL)
	}
}

func TestLocalProviderKeepsPinnedYAMLEndpoint(t *testing.T) {
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", t.TempDir())
	clearConfigEnv(t)

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "agent-harness.yml"),
		[]byte("provider: local\nendpoint_url: http://127.0.0.1:9000/v1\n"), 0644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	cfg, err := NewLayeredLoader(root).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.EndpointURL != "http://127.0.0.1:9000/v1" {
		t.Errorf("endpoint = %q, want yml-pinned local endpoint preserved", cfg.EndpointURL)
	}
}

func TestProjectYAMLDefaultWithoutUserSettings(t *testing.T) {
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", t.TempDir())
	clearConfigEnv(t)

	root := t.TempDir()
	writeProjectYAML(t, root)

	cfg, err := NewLayeredLoader(root).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Provider != "local" || cfg.Model != "ornith-q4" {
		t.Errorf("project defaults lost: provider = %q model = %q", cfg.Provider, cfg.Model)
	}
}

func TestLocalLayerOverridesUserSettings(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", configHome)
	clearConfigEnv(t)

	root := t.TempDir()
	writeProjectYAML(t, root)

	NewLayeredLoader(root).SaveSettings(SourceUser, map[string]interface{}{"provider": "openrouter", "model": "openai/gpt-4o"})

	localDir := filepath.Join(root, ".agent-harness")
	if err := os.MkdirAll(localDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "settings.local.json"),
		[]byte(`{"provider":"openai","model":"gpt-4o"}`), 0600); err != nil {
		t.Fatalf("write local settings: %v", err)
	}

	cfg, err := NewLayeredLoader(root).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Provider != "openai" || cfg.Model != "gpt-4o" {
		t.Errorf("local layer must win: provider = %q model = %q", cfg.Provider, cfg.Model)
	}
}

func TestSaveSettings_MergesExisting(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", configHome)
	clearConfigEnv(t)

	root := t.TempDir()
	ll := NewLayeredLoader(root)
	if err := ll.SaveSettings(SourceUser, map[string]interface{}{"provider": "openrouter"}); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if err := ll.SaveSettings(SourceUser, map[string]interface{}{"model": "openai/gpt-4o"}); err != nil {
		t.Fatalf("second save: %v", err)
	}

	cfg, err := NewLayeredLoader(root).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Provider != "openrouter" || cfg.Model != "openai/gpt-4o" {
		t.Errorf("merge failed: provider = %q model = %q", cfg.Provider, cfg.Model)
	}
}

func TestSaveSettings_FileModeIs0600(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", configHome)
	clearConfigEnv(t)

	ll := NewLayeredLoader(t.TempDir())
	if err := ll.SaveSettings(SourceUser, map[string]interface{}{"provider": "openai"}); err != nil {
		t.Fatalf("SaveSettings() error = %v", err)
	}

	info, err := os.Stat(filepath.Join(configHome, "settings.json"))
	if err != nil {
		t.Fatalf("stat settings.json: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("settings.json mode = %o, want 0600", perm)
	}
}
