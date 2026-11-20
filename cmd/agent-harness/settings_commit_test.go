package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/testharness/testenv"
)

// readUserSettings returns the keys of the user layer's settings.json.
func readUserSettings(t *testing.T) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(config.ConfigHome(), "settings.json"))
	if err != nil {
		t.Fatalf("read user settings: %v", err)
	}
	var values map[string]interface{}
	if err := json.Unmarshal(data, &values); err != nil {
		t.Fatalf("parse user settings: %v", err)
	}
	return values
}

// An environment pin is a per-session override. Saving an unrelated
// setting must not freeze the pinned provider, model, or endpoint into
// the user layer, where they would outlive the environment.
func TestCommitConfigChangeSkipsEnvPinnedValues(t *testing.T) {
	journeyHome(t)
	t.Setenv("AH_PROVIDER", "openrouter")
	t.Setenv("AH_MODEL", "journey/model-b")
	t.Setenv("AH_ENDPOINT_URL", "https://pinned.example/v1")
	t.Setenv("AH_API_KEY", "sk-pinned-test")

	app := bootJourneyApp(t)
	if app.config.Provider != "openrouter" || app.config.Model != "journey/model-b" || app.config.EndpointURL != "https://pinned.example/v1" {
		t.Fatalf("boot did not apply the pins: %s %s %s", app.config.Provider, app.config.Model, app.config.EndpointURL)
	}

	app.config.Theme = "nord"
	app.commitConfigChange()

	saved := readUserSettings(t)
	if saved["theme"] != "nord" {
		t.Fatalf("theme = %v, want nord (the unrelated change must persist)", saved["theme"])
	}
	for _, key := range []string{"provider", "model", "endpoint_url"} {
		if v, ok := saved[key]; ok {
			t.Errorf("env-pinned %s = %v was written to settings.json", key, v)
		}
	}
}

// Without a pin the same fields are the user's choice and persist.
func TestCommitConfigChangePersistsUnpinnedChoices(t *testing.T) {
	journeyHome(t)
	app := bootJourneyApp(t)
	app.config.Provider = "openrouter"
	app.config.Model = "chosen/model"
	app.config.EndpointURL = "https://chosen.example/v1"
	app.commitConfigChange()

	saved := readUserSettings(t)
	if saved["provider"] != "openrouter" || saved["model"] != "chosen/model" || saved["endpoint_url"] != "https://chosen.example/v1" {
		t.Fatalf("unpinned choices not persisted: %v", saved)
	}
}

func TestUserSettingsValuesRejectsBlankGuardedFields(t *testing.T) {
	cfg := &config.LayeredConfig{Theme: "nord", Tagline: ""}
	values, rejected := userSettingsValues(cfg)

	want := []string{"provider", "endpoint_url", "model", "context_length", "max_tokens"}
	if !reflect.DeepEqual(rejected, want) {
		t.Fatalf("rejected = %v, want %v", rejected, want)
	}
	for _, key := range want {
		if _, ok := values[key]; ok {
			t.Errorf("blank %s was kept in the write", key)
		}
	}
	for _, key := range []string{"temperature", "permission_mode", "tagline", "theme"} {
		if _, ok := values[key]; !ok {
			t.Errorf("%s missing from the write", key)
		}
	}
}

func TestUserSettingsValuesRejectsNonPositiveNumbers(t *testing.T) {
	cfg := &config.LayeredConfig{Provider: "local", Model: "m", EndpointURL: "http://x/v1", ContextLength: -1, MaxTokens: 0}
	values, rejected := userSettingsValues(cfg)
	if !reflect.DeepEqual(rejected, []string{"context_length", "max_tokens"}) {
		t.Fatalf("rejected = %v", rejected)
	}
	if _, ok := values["context_length"]; ok {
		t.Error("non-positive context_length kept")
	}
}

func TestUserSettingsValuesKeepsValidAndSkipsPinnedSilently(t *testing.T) {
	cfg := &config.LayeredConfig{
		Provider: "openrouter", Model: "m", EndpointURL: "http://x/v1",
		ContextLength: 8192, MaxTokens: 512,
		ModelPinned: true, EndpointPinned: true,
	}
	values, rejected := userSettingsValues(cfg)
	if len(rejected) != 0 {
		t.Fatalf("pinned fields reported as rejected: %v", rejected)
	}
	for _, key := range []string{"model", "endpoint_url"} {
		if _, ok := values[key]; ok {
			t.Errorf("pinned %s kept", key)
		}
	}
	if values["context_length"] != 8192 || values["max_tokens"] != 512 {
		t.Fatalf("valid numbers lost: %v", values)
	}
}

func TestPersistUserSettingsWarnsOnRejectedWrite(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "local", Model: "m", EndpointURL: "http://x/v1", ContextLength: 4096, MaxTokens: 128}, "m")
	app.cwd = t.TempDir()
	app.tuiApp = tui.NewApp()
	app.config.Model = ""
	app.persistUserSettings()

	msg := receiveTUIMessage(t, app.tuiApp)
	status, ok := msg.(tui.StatusMsg)
	if !ok || status.Type != "warning" || !strings.Contains(status.Text, "model") {
		t.Fatalf("message = %#v, want a warning naming model", msg)
	}
	if _, ok := readUserSettings(t)["model"]; ok {
		t.Fatal("blank model was written")
	}
}

// The handler-test constructor is the guard that keeps App tests off the
// developer's real config home: persistUserSettings and the credential
// store both resolve through config.ConfigHome, so an unisolated test
// rewrote ~/.config/agent-harness/settings.json. Simulate a shell that
// exports no override, point HOME at a sentinel, and require that a
// commit through the constructor leaves the sentinel untouched.
//
// This covers apps built through newHandlerTestApp. A test that builds
// &App{} by hand and commits without setting a config home is not caught.
func TestHandlerTestAppNeverWritesIntoHome(t *testing.T) {
	sentinel := t.TempDir()
	testenv.SetHome(t, sentinel)

	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "local", Model: "m", Theme: "nord"}, "m")
	app.cwd = t.TempDir()
	app.commitConfigChange()
	app.storeProviderModel("openrouter", "some/model")

	entries, err := os.ReadDir(sentinel)
	if err != nil {
		t.Fatalf("read sentinel home: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("test app wrote into the home directory: %v", entries)
	}
	if got := config.ConfigHome(); strings.HasPrefix(got, sentinel) {
		t.Fatalf("config home %q resolves inside HOME", got)
	}
}

// A pinned provider pins everything boot derived from it, and a pinned
// runtime stays out of the user layer too.
func TestUserSettingsValuesPinnedProviderSkipsDerivedValues(t *testing.T) {
	cfg := &config.LayeredConfig{
		Provider: "openrouter", Model: "m", EndpointURL: "http://x/v1", Runtime: "llama.cpp",
		ContextLength: 8192, MaxTokens: 512, ProviderPinned: true, RuntimePinned: true,
	}
	values, rejected := userSettingsValues(cfg)
	if len(rejected) != 0 {
		t.Fatalf("pinned values reported as rejected: %v", rejected)
	}
	for _, key := range []string{"provider", "model", "endpoint_url", "context_length", "max_tokens", "runtime"} {
		if _, ok := values[key]; ok {
			t.Errorf("%s kept under a provider/runtime pin", key)
		}
	}
}
