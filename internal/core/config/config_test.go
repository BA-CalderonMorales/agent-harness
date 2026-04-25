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
