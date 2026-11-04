package main

import (
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
)

// TestTemperatureSettingMarksZeroExplicit pins the live path: choosing
// 0.0 in Settings must change the very next request, not wait for a
// restart to reload the persisted value.
func TestTemperatureSettingMarksZeroExplicit(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "local", Temperature: 0.2}, "test-model")
	app.tuiApp = tui.NewApp()
	delegate := &tuiSettingsDelegate{app: app, tuiApp: app.tuiApp}

	delegate.OnSettingChange("temperature", "0")

	if app.config.Temperature != 0 {
		t.Fatalf("temperature = %v, want 0", app.config.Temperature)
	}
	if !app.config.TemperatureSet {
		t.Fatal("temperature 0 was not marked explicit; the next request would omit it")
	}
}
