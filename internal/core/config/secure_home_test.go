package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/testharness/testenv"
)

// The credential store must live under the same resolved config home as
// settings.json. It used to hardcode ~/.config/agent-harness, so
// AGENT_HARNESS_CONFIG_HOME (and XDG_CONFIG_HOME) relocated the settings
// while every profile kept sharing one credential file.
func TestSecureConfigPathFollowsConfigHome(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	t.Setenv("XDG_CONFIG_HOME", "")
	profile := filepath.Join(t.TempDir(), "work-profile")
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", profile)

	want := filepath.Join(profile, "credentials.enc")
	if got := SecureConfigPath(); got != want {
		t.Fatalf("SecureConfigPath = %q, want the profile store %q", got, want)
	}
	// The machine key is useless apart from its store and must not stay
	// behind in the shared home.
	if got := MachineKeyPath(); got != filepath.Join(profile, "master.key") {
		t.Fatalf("MachineKeyPath = %q, want it beside the profile store", got)
	}
	if got := LegacyConfigPath(); got != filepath.Join(profile, "config.json") {
		t.Fatalf("LegacyConfigPath = %q, want it under the profile too", got)
	}
}

func TestSecureConfigPathFollowsXDGConfigHome(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", "")
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	want := filepath.Join(xdg, "agent-harness", "credentials.enc")
	if got := SecureConfigPath(); got != want {
		t.Fatalf("SecureConfigPath = %q, want the XDG store %q", got, want)
	}
}

// With no override the store stays at the historical default root: fixing
// the override must not move the store for every existing user.
func TestSecureConfigPathDefaultLocationUnchanged(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	want := filepath.Join(home, ".config", "agent-harness", "credentials.enc")
	if got := SecureConfigPath(); got != want {
		t.Fatalf("SecureConfigPath = %q, want the default store %q", got, want)
	}
	if got := ConfigHome(); filepath.Dir(SecureConfigPath()) != got {
		t.Fatalf("store dir %q must equal ConfigHome %q", filepath.Dir(SecureConfigPath()), got)
	}
}

// Two profiles must not share a credential file: the whole point of the
// fix. Distinct config homes resolve to distinct stores.
func TestSecureConfigPathIsolatesProfiles(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	t.Setenv("XDG_CONFIG_HOME", "")

	t.Setenv("AGENT_HARNESS_CONFIG_HOME", filepath.Join(t.TempDir(), "a"))
	storeA := SecureConfigPath()
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", filepath.Join(t.TempDir(), "b"))
	storeB := SecureConfigPath()

	if storeA == storeB {
		t.Fatalf("two profiles resolved to one store: %q", storeA)
	}
}

// A store written before the fix lived at the literal default root even
// when an override was set. Switching to a profile must relocate it, or
// the keys already entered would be stranded and boot would fall to the
// login wall.
func TestAdoptLegacyCredentialStoreRelocatesKeys(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", "")

	// Seed at the default root (which is where the store lived pre-fix).
	cm := NewCredentialManager()
	if err := cm.SaveSecure(&SecureConfig{Provider: "openrouter", APIKey: "sk-or-real"}); err != nil {
		t.Fatalf("seed default-root store: %v", err)
	}
	legacyStore := filepath.Join(home, ".config", "agent-harness", "credentials.enc")
	if _, err := os.Stat(legacyStore); err != nil {
		t.Fatalf("seed store not at the default root: %v", err)
	}

	profile := filepath.Join(t.TempDir(), "work-profile")
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", profile)

	summary := AdoptLegacyCredentialStore()
	if summary == "" {
		t.Fatal("adoption with a populated legacy store should report a move")
	}
	if _, err := os.Stat(filepath.Join(profile, "credentials.enc")); err != nil {
		t.Fatalf("store did not move into the profile: %v", err)
	}
	if _, err := os.Stat(legacyStore); !os.IsNotExist(err) {
		t.Fatalf("legacy store should be gone after the move, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(profile, "master.key")); err != nil {
		t.Fatalf("master key did not travel with the store: %v", err)
	}

	// The relocated store must still decrypt: no stranded credentials.
	cfg, err := NewCredentialManager().LoadSecure()
	if err != nil {
		t.Fatalf("relocated store failed to unlock: %v", err)
	}
	if cfg.APIKey != "sk-or-real" || cfg.Provider != "openrouter" {
		t.Fatalf("relocated store mismatch: key=%q provider=%q", cfg.APIKey, cfg.Provider)
	}

	// Idempotent: a second run has nothing left to adopt.
	if again := AdoptLegacyCredentialStore(); again != "" {
		t.Fatalf("second adoption should be a no-op, got %q", again)
	}
}

// A profile that already logged in has its own store; adoption must never
// overwrite it with the shared legacy file.
func TestAdoptLegacyCredentialStoreKeepsProfileStore(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", "")

	legacyDir := filepath.Join(home, ".config", "agent-harness")
	if err := os.MkdirAll(legacyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	legacyStore := filepath.Join(legacyDir, "credentials.enc")
	if err := os.WriteFile(legacyStore, []byte("legacy-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}

	profile := filepath.Join(t.TempDir(), "work-profile")
	if err := os.MkdirAll(profile, 0o700); err != nil {
		t.Fatal(err)
	}
	profileStore := filepath.Join(profile, "credentials.enc")
	if err := os.WriteFile(profileStore, []byte("profile-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", profile)

	if summary := AdoptLegacyCredentialStore(); summary != "" {
		t.Fatalf("adoption must not run when a profile store exists, got %q", summary)
	}
	got, err := os.ReadFile(profileStore)
	if err != nil {
		t.Fatalf("read profile store: %v", err)
	}
	if string(got) != "profile-bytes" {
		t.Fatalf("profile store was overwritten: %q", got)
	}
	if _, err := os.Stat(legacyStore); err != nil {
		t.Fatalf("legacy store must be left in place, stat err = %v", err)
	}
}

// No override means the store already lives at the resolved home, so
// adoption is a no-op and the store never moves.
func TestAdoptLegacyCredentialStoreNoopAtDefaultHome(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", "")

	cm := NewCredentialManager()
	if err := cm.SaveSecure(&SecureConfig{Provider: "openrouter", APIKey: "sk-or-real"}); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	if summary := AdoptLegacyCredentialStore(); summary != "" {
		t.Fatalf("adoption at the default home must be a no-op, got %q", summary)
	}
	if _, err := os.Stat(SecureConfigPath()); err != nil {
		t.Fatalf("store should stay put: %v", err)
	}
}
