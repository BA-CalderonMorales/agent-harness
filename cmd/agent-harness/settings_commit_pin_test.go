package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
)

// providerDerivedKeys are the values boot derives from the provider's
// defaults when AH_PROVIDER is pinned. They describe the provider, so a
// pinned provider must not leave them persisted beside a different one.
var providerDerivedKeys = []string{"provider", "endpoint_url", "model", "context_length", "max_tokens", "runtime"}

// compatSettingsFixture is resolved at package init, while the working
// directory is still the package dir; journeyHome changes it later.
var compatSettingsFixture, _ = filepath.Abs(filepath.Join("..", "..", "internal", "core", "config", "testdata", "compat", "settings-v0.4.1.json"))

// seedCompatUserSettings installs the v0.4.1 compat fixture (a lemonade
// user) as the user layer and returns its parsed keys.
func seedCompatUserSettings(t *testing.T) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(compatSettingsFixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	path := filepath.Join(config.ConfigHome(), "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return readUserSettings(t)
}

func assertDerivedKeysUnchanged(t *testing.T, before, after map[string]interface{}) {
	t.Helper()
	for _, key := range providerDerivedKeys {
		if !reflect.DeepEqual(before[key], after[key]) {
			t.Errorf("%s = %v after the write, user file had %v", key, after[key], before[key])
		}
	}
}

// Only AH_PROVIDER is pinned. Boot fills endpoint, model and context from
// the pinned provider's defaults in memory; persisting them next to the
// user's own provider wrote a mismatched pair (lemonade + openrouter's
// endpoint and model).
func TestCommitConfigChangeKeepsUserFileWhileProviderPinned(t *testing.T) {
	journeyHome(t)
	before := seedCompatUserSettings(t)
	t.Setenv("AH_PROVIDER", "openrouter")
	t.Setenv("AH_API_KEY", "sk-pinned-test")

	app := bootJourneyApp(t)
	if app.config.Provider != "openrouter" {
		t.Fatalf("boot provider = %q, want the pin", app.config.Provider)
	}
	app.config.Theme = "nord"
	app.commitConfigChange()

	after := readUserSettings(t)
	if after["theme"] != "nord" {
		t.Fatalf("theme = %v, want nord", after["theme"])
	}
	assertDerivedKeysUnchanged(t, before, after)
}
