package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
)

// The settings pipeline must keep its promises: a toggle in the
// Settings tab persists to disk, applies at runtime, and an explicit
// toggle outranks the permission-mode preset — the reported failure
// was execute enabled in settings while ls and Shell still denied.

func delegateFor(t *testing.T, app *App) *tuiSettingsDelegate {
	t.Helper()
	app.tuiApp = tui.NewApp()
	delegate := &tuiSettingsDelegate{app: app, tuiApp: app.tuiApp}
	return delegate
}

func pressSetting(app *App, delegate *tuiSettingsDelegate, key, value string) {
	delegate.OnSettingChange(key, value)
}

func decisionFor(app *App, toolName string) tools.PermissionDecision {
	return app.checkPermissionMode(toolName)
}

// TestExecuteToggleAppliesAtRuntime: flipping Execute on in settings
// must let bash past the permission-mode deny — glob and Read worked
// while ls and Shell denied, because the mode preset overrode the
// granular toggle the user had just set.
func TestExecuteToggleAppliesAtRuntime(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{
		Provider:       "openrouter",
		PermissionMode: config.PermissionReadOnly,
		PermRead:       true,
	}, "demo-1.0")
	delegate := delegateFor(t, app)

	// Before the toggle: bash denies under the read-only preset.
	if d := decisionFor(app, "bash"); d.Behavior != tools.Deny {
		t.Fatalf("bash allowed before any toggle: %v", d.Behavior)
	}

	pressSetting(app, delegate, "perm_execute", "true")

	if !app.config.PermExecute {
		t.Fatal("perm_execute toggle did not reach the config")
	}
	if !app.config.PermExplicit {
		t.Fatal("explicit toggle did not mark the config")
	}
	d := decisionFor(app, "bash")
	if d.Behavior == tools.Deny {
		t.Fatalf("execute toggle did not outrank the read-only preset: %v (%s)", d.Behavior, d.Message)
	}

	// Read tools still governed by their own toggle.
	pressSetting(app, delegate, "perm_read", "false")
	if d := decisionFor(app, "read"); d.Behavior != tools.Deny {
		t.Fatalf("read deny lost after toggling read off: %v", d.Behavior)
	}
}

// TestPermissionTogglePersists: the toggle must reach the persisted
// settings — the reported failure was permissions vanishing on reopen
// of the TUI.
func TestPermissionTogglePersists(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", configHome)
	app := newHandlerTestApp(t, &config.LayeredConfig{
		Provider:       "openrouter",
		PermissionMode: config.PermissionReadOnly,
		PermRead:       true,
	}, "demo-1.0")
	delegate := delegateFor(t, app)

	pressSetting(app, delegate, "perm_execute", "true")

	// The user-settings save wrote the permission state: the settings
	// file on disk must carry execute=true for the next launch.
	data, err := os.ReadFile(filepath.Join(configHome, "settings.json"))
	if err != nil {
		t.Fatalf("read saved settings: %v", err)
	}
	var saved map[string]interface{}
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("parse saved settings: %v", err)
	}
	if v, ok := saved["perm_execute"]; !ok || v != true {
		t.Fatalf("saved settings missing perm_execute=true: %v", saved)
	}
}

// TestPermissionModeResetClearsExplicit: choosing a preset re-owns the
// granular toggles — an explicit toggle from before the switch must
// not override the preset just picked.
func TestPermissionModeResetClearsExplicit(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{
		Provider:       "openrouter",
		PermissionMode: config.PermissionReadOnly,
		PermRead:       true,
	}, "demo-1.0")
	delegate := delegateFor(t, app)

	pressSetting(app, delegate, "perm_execute", "true")
	pressSetting(app, delegate, "permissions", "read-only")

	if app.config.PermExplicit {
		t.Fatal("preset switch left the explicit flag set")
	}
	if app.config.PermExecute {
		t.Fatal("preset switch did not re-sync the execute toggle")
	}
	if d := decisionFor(app, "bash"); d.Behavior != tools.Deny {
		t.Fatalf("read-only preset must deny bash after reset: %v", d.Behavior)
	}
}

// TestGranularDenyBeatsMode: an explicit off denies even when the mode
// preset would allow — the toggles are the authority both ways.
func TestGranularDenyBeatsMode(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{
		Provider:       "openrouter",
		PermissionMode: config.PermissionWorkspaceWrite,
		PermRead:       true,
		PermWrite:      true,
	}, "demo-1.0")
	delegate := delegateFor(t, app)

	pressSetting(app, delegate, "perm_execute", "false")
	pressSetting(app, delegate, "perm_read", "false")

	if d := decisionFor(app, "bash"); d.Behavior != tools.Deny || !strings.Contains(d.Message, "BASH") {
		t.Fatalf("execute deny lost: %v (%s)", d.Behavior, d.Message)
	}
	if d := decisionFor(app, "read"); d.Behavior != tools.Deny || !strings.Contains(d.Message, "READ") && !strings.Contains(d.Message, "READ permission") {
		t.Fatalf("read deny lost: %v (%s)", d.Behavior, d.Message)
	}
	// Unmapped tools fall through to the mode preset.
	if d := decisionFor(app, "write"); d.Behavior == tools.Deny {
		t.Fatalf("unmapped tool wrongly caught by a granular deny: %v", d.Behavior)
	}
}
