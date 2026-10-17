package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
)

// TestPersistUserSettingsNeverBlankProvider pins the writer's half of the
// delta contract: a value that carries no information must not be written
// to the user layer at all.
//
// Writing `"provider": ""` looked harmless and was not. The reader drops
// empty strings as no-ops, so the key stopped carrying a value and the
// next boot fell back to the default provider — `local`, and the login
// wall behind an unreachable local probe — while the user's real choice
// was gone from the file. Blanking the key has to be prevented at the
// writer, because by the time the reader sees it the choice is lost.
func TestPersistUserSettingsNeverBlankProvider(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", home)

	// A config that has been drained of value — the shape a sparse or
	// test-built App reaches this path with.
	app := &App{
		cwd: t.TempDir(),
		config: &config.LayeredConfig{
			Provider: "nvidia",
			Model:    "deepseek-ai/deepseek-v4-flash-0731",
		},
	}
	app.persistUserSettings()

	path := filepath.Join(home, "settings.json")
	seeded := readSettingsFile(t, path)
	if seeded["provider"] != "nvidia" {
		t.Fatalf("provider = %v, want the chosen provider persisted", seeded["provider"])
	}

	// The provider is now unset in the live config: the write must leave
	// the stored value alone rather than freeze an empty override over it.
	app.config.Provider = ""
	app.config.Runtime = ""
	app.config.Effort = ""
	app.persistUserSettings()

	after := readSettingsFile(t, path)
	if v, ok := after["provider"]; ok && v == "" {
		t.Fatalf("provider was blanked to %q: an empty override is dropped on read and the user's provider is lost", v)
	}
	if after["provider"] != "nvidia" {
		t.Fatalf("provider = %v, want the earlier choice retained", after["provider"])
	}
	for _, key := range []string{"runtime", "reasoning_effort"} {
		if v, ok := after[key]; ok && v == "" {
			t.Fatalf("%s was blanked to an empty override", key)
		}
	}
}

// readSettingsFile parses a settings layer as the loader would.
func readSettingsFile(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	values := map[string]interface{}{}
	if err := json.Unmarshal(data, &values); err != nil {
		t.Fatalf("Unmarshal(%s): %v", path, err)
	}
	return values
}
