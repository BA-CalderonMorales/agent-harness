package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFile_MissingReturnsEmpty(t *testing.T) {
	cfg, err := LoadFile("/nonexistent/path/config.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Provider != "" {
		t.Errorf("expected empty provider, got %q", cfg.Provider)
	}
}

func TestLoadFile_ReadsValidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")
	data := `{"provider": "openai", "model": "gpt-4o", "verbose": true}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Provider != "openai" {
		t.Errorf("provider = %q, want openai", cfg.Provider)
	}
	if cfg.Model != "gpt-4o" {
		t.Errorf("model = %q, want gpt-4o", cfg.Model)
	}
	if !cfg.Verbose {
		t.Error("expected verbose true")
	}
}

func TestFileConfig_SaveAndLoadRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")

	original := &FileConfig{
		Provider:    "anthropic",
		Model:       "claude-3-5-sonnet-20241022",
		AlwaysAllow: []string{"read", "glob"},
	}

	if err := original.Save(path); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.Provider != original.Provider {
		t.Errorf("provider = %q, want %q", loaded.Provider, original.Provider)
	}
	if len(loaded.AlwaysAllow) != 2 {
		t.Errorf("always_allow len = %d, want 2", len(loaded.AlwaysAllow))
	}
}

func TestLayeredLoader_DefaultsToLocalOrnith(t *testing.T) {
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", t.TempDir())
	clearConfigEnv(t)

	cfg, err := NewLayeredLoader(t.TempDir()).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Provider != DefaultProvider {
		t.Fatalf("provider = %q, want %q", cfg.Provider, DefaultProvider)
	}
	if cfg.Runtime != DefaultRuntime {
		t.Fatalf("runtime = %q, want %q", cfg.Runtime, DefaultRuntime)
	}
	if cfg.Model != DefaultModel {
		t.Fatalf("model = %q, want %q", cfg.Model, DefaultModel)
	}
	if cfg.EndpointURL != DefaultEndpointURL {
		t.Fatalf("endpoint = %q, want %q", cfg.EndpointURL, DefaultEndpointURL)
	}
	if cfg.ContextLength != DefaultContextLength {
		t.Fatalf("context_length = %d, want %d", cfg.ContextLength, DefaultContextLength)
	}
	if cfg.MaxTokens != DefaultMaxTokens {
		t.Fatalf("max_tokens = %d, want %d", cfg.MaxTokens, DefaultMaxTokens)
	}
}

func TestLayeredLoader_ReadsRootYAMLConfig(t *testing.T) {
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", t.TempDir())
	clearConfigEnv(t)

	root := t.TempDir()
	data := []byte(`
provider: local
runtime: llama.cpp
model: ornith-q4
model_path: ~/models/ornith-q4.gguf
endpoint_url: http://127.0.0.1:9000/v1
context_length: 16384
temperature: 0.15
max_tokens: 2048
workspace_path: /workspace
local_server_command: llama-server -m ~/models/ornith-q4.gguf --port 9000
permissions:
  mode: workspace-write
  read: true
  write: true
  delete: false
  execute: false
`)
	if err := os.WriteFile(filepath.Join(root, "agent-harness.yml"), data, 0644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	cfg, err := NewLayeredLoader(root).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Model != "ornith-q4" || cfg.ModelPath != "~/models/ornith-q4.gguf" {
		t.Fatalf("model config = %q path %q", cfg.Model, cfg.ModelPath)
	}
	if cfg.EndpointURL != "http://127.0.0.1:9000/v1" {
		t.Fatalf("endpoint = %q", cfg.EndpointURL)
	}
	if cfg.ContextLength != 16384 || cfg.MaxTokens != 2048 || cfg.Temperature != 0.15 {
		t.Fatalf("generation config = context:%d max:%d temp:%v", cfg.ContextLength, cfg.MaxTokens, cfg.Temperature)
	}
	if cfg.PermissionMode != PermissionWorkspaceWrite {
		t.Fatalf("permission mode = %s, want workspace-write", cfg.PermissionMode)
	}
	if !cfg.PermExplicit || !cfg.PermRead || !cfg.PermWrite || cfg.PermDelete || cfg.PermExecute {
		t.Fatalf("permission toggles = explicit:%v read:%v write:%v delete:%v execute:%v",
			cfg.PermExplicit, cfg.PermRead, cfg.PermWrite, cfg.PermDelete, cfg.PermExecute)
	}
}

func TestLayeredLoader_EnvironmentOverridesYAML(t *testing.T) {
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", t.TempDir())
	clearConfigEnv(t)
	t.Setenv("AH_PROVIDER", "openrouter")
	t.Setenv("AH_MODEL", "openai/gpt-4o")
	t.Setenv("AH_ENDPOINT_URL", "http://proxy.local/v1")
	t.Setenv("AH_MAX_TOKENS", "1234")
	t.Setenv("AH_TEMPERATURE", "0.33")
	t.Setenv("AH_PERM_EXECUTE", "true")

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "agent-harness.yml"), []byte("provider: local\nmodel: local-model\n"), 0644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	cfg, err := NewLayeredLoader(root).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Provider != "openrouter" || cfg.Model != "openai/gpt-4o" {
		t.Fatalf("provider/model = %q/%q", cfg.Provider, cfg.Model)
	}
	if cfg.EndpointURL != "http://proxy.local/v1" {
		t.Fatalf("endpoint = %q", cfg.EndpointURL)
	}
	if cfg.MaxTokens != 1234 || cfg.Temperature != 0.33 {
		t.Fatalf("generation config = max:%d temp:%v", cfg.MaxTokens, cfg.Temperature)
	}
	if !cfg.PermExplicit || !cfg.PermExecute {
		t.Fatalf("execute permission override not applied")
	}
}

func TestLayeredLoader_RemoteProviderGetsProviderDefaultModel(t *testing.T) {
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", t.TempDir())
	clearConfigEnv(t)
	t.Setenv("AH_PROVIDER", "openai")

	cfg, err := NewLayeredLoader(t.TempDir()).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Model != "gpt-4o" {
		t.Fatalf("model = %q, want gpt-4o", cfg.Model)
	}
	if cfg.EndpointURL != "https://api.openai.com/v1" {
		t.Fatalf("endpoint = %q, want provider default https://api.openai.com/v1", cfg.EndpointURL)
	}
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"AH_PROVIDER", "AGENT_HARNESS_PROVIDER",
		"AH_RUNTIME", "AGENT_HARNESS_RUNTIME",
		"AH_MODEL", "AGENT_HARNESS_MODEL",
		"AH_MODEL_PATH", "AGENT_HARNESS_MODEL_PATH",
		"AH_ENDPOINT_URL", "AGENT_HARNESS_ENDPOINT_URL",
		"AH_API_KEY", "AGENT_HARNESS_API_KEY",
		"AH_CONTEXT_LENGTH", "AGENT_HARNESS_CONTEXT_LENGTH",
		"AH_TEMPERATURE", "AGENT_HARNESS_TEMPERATURE",
		"AH_MAX_TOKENS", "AGENT_HARNESS_MAX_TOKENS",
		"AH_WORKSPACE_PATH", "AGENT_HARNESS_WORKSPACE_PATH",
		"AH_LOCAL_SERVER_COMMAND", "AGENT_HARNESS_LOCAL_SERVER_COMMAND",
		"AH_PERSONA", "AGENT_HARNESS_PERSONA",
		"AH_PERMISSION_MODE", "AGENT_HARNESS_PERMISSION_MODE",
		"AH_EXECUTION_MODE", "AGENT_HARNESS_EXECUTION_MODE",
		"AH_PERM_READ", "AGENT_HARNESS_PERM_READ",
		"AH_PERM_WRITE", "AGENT_HARNESS_PERM_WRITE",
		"AH_PERM_DELETE", "AGENT_HARNESS_PERM_DELETE",
		"AH_PERM_EXECUTE", "AGENT_HARNESS_PERM_EXECUTE",
	} {
		t.Setenv(key, "")
	}
}
