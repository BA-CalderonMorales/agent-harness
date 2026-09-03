package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDataHomePrecedence(t *testing.T) {
	t.Setenv("AGENT_HARNESS_DATA_HOME", "/tmp/ah-data-override")
	if got := DataHome(); got != "/tmp/ah-data-override" {
		t.Fatalf("DataHome = %q, want the explicit override", got)
	}

	t.Setenv("AGENT_HARNESS_DATA_HOME", "")
	t.Setenv("XDG_DATA_HOME", "/xdg/data")
	if got := DataHome(); got != "/xdg/data/agent-harness" {
		t.Fatalf("DataHome = %q, want the XDG path", got)
	}

	t.Setenv("XDG_DATA_HOME", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	want := filepath.Join(home, ".local", "share", "agent-harness")
	if got := DataHome(); got != want {
		t.Fatalf("DataHome = %q, want %q", got, want)
	}
}

func TestConfigHomePrecedence(t *testing.T) {
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", "/tmp/ah-config-override")
	if got := ConfigHome(); got != "/tmp/ah-config-override" {
		t.Fatalf("ConfigHome = %q, want the explicit override", got)
	}

	t.Setenv("AGENT_HARNESS_CONFIG_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "/xdg/config")
	if got := ConfigHome(); got != "/xdg/config/agent-harness" {
		t.Fatalf("ConfigHome = %q, want the XDG path", got)
	}
}

func TestMigrateLegacyHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	legacy := filepath.Join(home, ".agent-harness")
	for _, dir := range []string{"sessions", "audit", "logs", "tool-results"} {
		if err := os.MkdirAll(filepath.Join(legacy, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(legacy, "settings.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "unknown-artifact.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	summary := MigrateLegacyHome()
	if summary == "" {
		t.Fatal("migration with a legacy home should return a summary")
	}

	// Known children moved into the XDG homes.
	for _, dir := range []string{"sessions", "audit", "logs", "tool-results"} {
		if _, err := os.Stat(filepath.Join(DataHome(), dir)); err != nil {
			t.Errorf("%s did not move to DataHome: %v", dir, err)
		}
	}
	if _, err := os.Stat(filepath.Join(ConfigHome(), "settings.json")); err != nil {
		t.Errorf("settings.json did not move to ConfigHome: %v", err)
	}
	// Unknown entries stay put — the migration never deletes what it
	// does not own.
	if _, err := os.Stat(filepath.Join(legacy, "unknown-artifact.txt")); err != nil {
		t.Error("unknown legacy entries must be left in place")
	}

	// Idempotent: a second run on a migrated (empty) home is a no-op.
	if again := MigrateLegacyHome(); again != "" {
		t.Fatalf("second migration should be a no-op, got %q", again)
	}
}

func TestMigrateLegacyHome_NeverClobbersNewHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	legacy := filepath.Join(home, ".agent-harness", "settings.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte(`{"old":true}`), 0o600); err != nil {
		t.Fatal(err)
	}

	fresh := filepath.Join(ConfigHome(), "settings.json")
	if err := os.MkdirAll(ConfigHome(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fresh, []byte(`{"new":true}`), 0o600); err != nil {
		t.Fatal(err)
	}

	MigrateLegacyHome()

	data, err := os.ReadFile(fresh)
	if err != nil {
		t.Fatalf("read new settings: %v", err)
	}
	if string(data) != `{"new":true}` {
		t.Fatalf("new-home settings clobbered by legacy copy: %q", data)
	}
}
