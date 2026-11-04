package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Reduced motion is read from the same layers as every other setting:
// the config files first, then the environment, which wins.

func TestReducedMotionFromSettingsFile(t *testing.T) {
	dir := t.TempDir()
	// Isolate the user layer so the developer's own settings cannot
	// decide the outcome.
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", filepath.Join(dir, "config-home"))
	settings := filepath.Join(dir, ".agent-harness", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(settings, []byte(`{"reduced_motion": true}`), 0o600); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	t.Setenv("AH_REDUCED_MOTION", "")
	t.Setenv("AGENT_HARNESS_REDUCED_MOTION", "")

	loader := NewLayeredLoader(dir)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.ReducedMotion {
		t.Fatal("ReducedMotion = false, want the settings value to apply")
	}
}

func TestReducedMotionFromEnvironment(t *testing.T) {
	dir := t.TempDir()
	// Isolate the user layer so the developer's own settings cannot
	// decide the outcome.
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", filepath.Join(dir, "config-home"))
	settings := filepath.Join(dir, ".agent-harness", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(settings, []byte(`{"reduced_motion": false}`), 0o600); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	t.Setenv("AGENT_HARNESS_REDUCED_MOTION", "1")

	loader := NewLayeredLoader(dir)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.ReducedMotion {
		t.Fatal("ReducedMotion = false, want the environment to win")
	}
}

func TestReducedMotionDefaultsToOff(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", filepath.Join(dir, "config-home"))
	t.Setenv("AH_REDUCED_MOTION", "")
	t.Setenv("AGENT_HARNESS_REDUCED_MOTION", "")

	loader := NewLayeredLoader(dir)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ReducedMotion {
		t.Fatal("ReducedMotion = true by default, want motion unless asked otherwise")
	}
}
