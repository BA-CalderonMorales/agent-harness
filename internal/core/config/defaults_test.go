package config

import (
	"os"
	"path/filepath"
	"testing"
)

// The default local model on the Lemonade path. gemma4-it-12b-FLM passed
// 11 of 15 automatic eval runs against qwen3.5-9b's 5 of 15, so it is the
// shipped default. An explicit AH_MODEL or saved settings value must still
// win over it.
const wantDefaultLemonadeModel = "gemma4-it-12b-FLM"

func TestDefaultLemonadeModelIsGemma4(t *testing.T) {
	if DefaultLemonadeModel != wantDefaultLemonadeModel {
		t.Fatalf("DefaultLemonadeModel = %q, want %q", DefaultLemonadeModel, wantDefaultLemonadeModel)
	}
	if got := DefaultModelForProvider("lemonade"); got != wantDefaultLemonadeModel {
		t.Fatalf("DefaultModelForProvider(lemonade) = %q, want %q", got, wantDefaultLemonadeModel)
	}
}

func TestLemonadeProviderLoadsGemma4ByDefault(t *testing.T) {
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", t.TempDir())
	clearConfigEnv(t)
	t.Setenv("AH_PROVIDER", "lemonade")

	cfg, err := NewLayeredLoader(t.TempDir()).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Model != wantDefaultLemonadeModel {
		t.Fatalf("model = %q, want default %q", cfg.Model, wantDefaultLemonadeModel)
	}
}

func TestExplicitAHModelWinsOverLemonadeDefault(t *testing.T) {
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", t.TempDir())
	clearConfigEnv(t)
	t.Setenv("AH_PROVIDER", "lemonade")
	t.Setenv("AH_MODEL", "qwen3.5-9b-FLM")

	cfg, err := NewLayeredLoader(t.TempDir()).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Model != "qwen3.5-9b-FLM" {
		t.Fatalf("model = %q, want explicit AH_MODEL qwen3.5-9b-FLM", cfg.Model)
	}
}

func TestSavedLemonadeModelWinsOverDefault(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", configHome)
	clearConfigEnv(t)
	saved := `{"provider": "lemonade", "model": "qwen3.5-9b-FLM", "endpoint_url": "http://127.0.0.1:13305/v1"}`
	if err := os.WriteFile(filepath.Join(configHome, "settings.json"), []byte(saved), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := NewLayeredLoader(t.TempDir()).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Model != "qwen3.5-9b-FLM" {
		t.Fatalf("model = %q, want saved qwen3.5-9b-FLM", cfg.Model)
	}
}
