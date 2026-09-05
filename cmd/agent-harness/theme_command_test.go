package main

import (
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
)

// TestThemeCommandCycles pins bare /theme: it advances to the next
// palette in the catalog and wraps. A valid name applies and persists,
// an unknown name errors.
func TestThemeCommandCycles(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "local"}, "test-model")
	app.initCommandsCore()
	reg := app.cmdRegistry

	out, handled, err := reg.Handle("/theme")
	if !handled || err != nil {
		t.Fatalf("/theme not handled: handled=%v err=%v", handled, err)
	}
	names := tui.ThemeNames()
	// ThemeNames pins "default" first; the cycle wraps to the next entry.
	want := names[1]
	if app.config.Theme != want {
		t.Fatalf("bare /theme applied %q, want %q (next after default)", app.config.Theme, want)
	}
	if !strings.Contains(out, want) {
		t.Fatalf("/theme output missing %q:\n%s", want, out)
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
