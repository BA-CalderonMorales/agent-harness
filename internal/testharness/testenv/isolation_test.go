package testenv_test

import (
	"path/filepath"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/testharness/testenv"
)

func TestSetHomeRedirectsInheritedStorageRoots(t *testing.T) {
	for _, variable := range []string{"AGENT_HARNESS_CONFIG_HOME", "AGENT_HARNESS_DATA_HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "AGENT_HARNESS_SESSION_DIR"} {
		t.Setenv(variable, filepath.Join(t.TempDir(), "external"))
	}
	home := t.TempDir()
	testenv.SetHome(t, home)
	if got, want := config.ConfigHome(), filepath.Join(home, ".config", "agent-harness"); got != want {
		t.Fatalf("fixture still uses external configuration: %q, want %q", got, want)
	}
	if got, want := config.DataHome(), filepath.Join(home, ".local", "share", "agent-harness"); got != want {
		t.Fatalf("fixture still uses external data: %q, want %q", got, want)
	}
}
