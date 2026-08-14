package config

import (
	"os"
	"testing"
)

// TestLoadSecureThenSaveSecureRoundTrip pins the store-bricking bug: a
// manager that LoadSecure'd (caching a key derived with the old salt)
// must not encrypt a follow-up SaveSecure under that stale key while
// writing a fresh salt — the next boot could never decrypt it.
func TestLoadSecureThenSaveSecureRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cm := NewCredentialManager()
	if err := cm.SaveSecure(&SecureConfig{Provider: "openrouter", APIKey: "sk-test-123", Model: "m"}); err != nil {
		t.Fatalf("initial SaveSecure: %v", err)
	}
	if _, err := os.Stat(MachineKeyPath()); err != nil {
		t.Fatalf("machine key not created: %v", err)
	}

	// Load, then re-save on the SAME manager (the retention flow in
	// completeLogin and persistAPIKey do exactly this).
	cfg, err := cm.LoadSecure()
	if err != nil {
		t.Fatalf("LoadSecure: %v", err)
	}
	if cfg.APIKey != "sk-test-123" {
		t.Fatalf("round-trip key mismatch: %q", cfg.APIKey)
	}
	cfg.Provider = "anthropic"
	if err := cm.SaveSecure(cfg); err != nil {
		t.Fatalf("SaveSecure after LoadSecure: %v", err)
	}

	// A fresh manager must still decrypt — this is the next boot.
	fresh := NewCredentialManager()
	cfg2, err := fresh.LoadSecure()
	if err != nil {
		t.Fatalf("fresh LoadSecure after save cycle: %v", err)
	}
	if cfg2.APIKey != "sk-test-123" || cfg2.Provider != "anthropic" {
		t.Fatalf("fresh round-trip mismatch: key=%q provider=%q", cfg2.APIKey, cfg2.Provider)
	}
}
