package main

import (
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
)

// TestModesCommandListsAllFourAndMarksActive pins /modes behavior.
func TestModesCommandListsAllFourAndMarksActive(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "local"}, "test-model")
	app.initCommandsCore()
	app.syncAgentMode() // boot derives manual (interactive) as the default
	reg := app.cmdRegistry

	out, handled, err := reg.Handle("/modes")
	if !handled || err != nil {
		t.Fatalf("/modes not handled: handled=%v err=%v", handled, err)
	}
	for _, want := range []string{"manual", "auto", "plan", "chat"} {
		if !strings.Contains(out, want) {
			t.Fatalf("/modes missing %q:\n%s", want, out)
		}
	}
	if strings.Count(out, "→") != 1 {
		t.Fatalf("/modes must mark exactly one active mode:\n%s", out)
	}
	if !strings.Contains(out, "→ manual") {
		t.Fatalf("default mode manual not marked:\n%s", out)
	}

	// Switching the mode moves the marker.
	app.applyAgentMode(AgentModePlan)
	out, _, _ = reg.Handle("/modes")
	if !strings.Contains(out, "→ plan") || strings.Contains(out, "→ manual") {
		t.Fatalf("marker did not follow the mode switch:\n%s", out)
	}
}

// TestHelpGroupsModesAndFlags pins the /help contract: a Modes category,
// one line per command from the registry descriptions, and feature-
// flagged commands visible under Coming soon.
func TestHelpGroupsModesAndFlags(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "local"}, "test-model")
	app.initCommandsCore()
	app.cmdRegistry.FeatureFlag("todo", "Persistent task lists")

	out, handled, err := app.cmdRegistry.Handle("/help")
	if !handled || err != nil {
		t.Fatalf("/help not handled: handled=%v err=%v", handled, err)
	}
	if !strings.Contains(out, "Modes:") {
		t.Fatalf("/help missing Modes category:\n%s", out)
	}
	if !strings.Contains(out, "/modes") {
		t.Fatalf("/help missing /modes:\n%s", out)
	}
	if !strings.Contains(out, "Coming soon:") || !strings.Contains(out, "/todo") {
		t.Fatalf("/help missing feature-flag section:\n%s", out)
	}
}
