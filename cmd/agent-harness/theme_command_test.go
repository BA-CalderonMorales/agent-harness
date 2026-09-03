package main

import (
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
)

// TestThemeCommandListsAndApplies pins /theme: no args lists the
// catalog with the active marker, a valid name applies and persists,
// an unknown name errors.
func TestThemeCommandListsAndApplies(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "local"}, "test-model")
	app.initCommandsCore()
	reg := app.cmdRegistry

	out, handled, err := reg.Handle("/theme")
	if !handled || err != nil {
		t.Fatalf("/theme not handled: handled=%v err=%v", handled, err)
	}
	if !strings.Contains(out, "→ default") || !strings.Contains(out, "nord") {
		t.Fatalf("/theme listing wrong:\n%s", out)
	}

	out, _, err = reg.Handle("/theme nord")
	if err != nil || !strings.Contains(out, "nord") {
		t.Fatalf("/theme nord failed: out=%q err=%v", out, err)
	}
	if app.config.Theme != "nord" {
		t.Fatalf("theme not persisted to config: %q", app.config.Theme)
	}

	if _, _, err := reg.Handle("/theme bogus"); err == nil {
		t.Fatal("unknown theme did not error")
	}
}

// TestThemeSettingRowPinsCatalog pins the Settings row: the theme choice
// offers the full catalog and shows the active value.
func TestThemeSettingRowPinsCatalog(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "local", Theme: "nord"}, "test-model")

	var themeRow *tui.Setting
	settings := app.getSettings()
	for i := range settings {
		if settings[i].Key == "theme" {
			themeRow = &settings[i]
			break
		}
	}
	if themeRow == nil {
		t.Fatal("Settings has no theme row")
	}
	if themeRow.Value != "nord" {
		t.Fatalf("theme row value = %q, want nord", themeRow.Value)
	}
	if len(themeRow.Options) != 20 {
		t.Fatalf("theme row offers %d options, want 20", len(themeRow.Options))
	}
}
