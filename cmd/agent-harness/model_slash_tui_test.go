package main

import (
	"reflect"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
)

// TestModelSlashCommandSendsModelChangedMsg pins the composer mode-line
// fix: the /model slash path must notify the TUI (ModelChangedMsg), not
// just mutate the session — the mode line reads chatModel.model, which
// only ModelChangedMsg updates.
func TestModelSlashCommandSendsModelChangedMsg(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "local"}, "old-model")
	app.tuiApp = tui.NewApp()
	app.initCommandsCore()

	if _, _, err := app.cmdRegistry.Handle("/model new-model"); err != nil {
		t.Fatalf("/model new-model error = %v", err)
	}

	msg := receiveTUIMessage(t, app.tuiApp)
	changed, ok := msg.(tui.ModelChangedMsg)
	if !ok {
		t.Fatalf("first TUI message = %T, want tui.ModelChangedMsg", msg)
	}
	if changed.Model != "new-model" {
		t.Fatalf("ModelChangedMsg.Model = %q, want new-model", changed.Model)
	}

	// The TUI side must actually land it in the chat model: the mode
	// line renders from chatModel.model.
	app.tuiApp.Update(msg)
	chatModel := exportedField(reflect.ValueOf(app.tuiApp).Elem().FieldByName("chatModel"))
	got := exportedField(chatModel.FieldByName("model")).String()
	if got != "new-model" {
		t.Fatalf("chatModel.model = %q after slash switch, want new-model", got)
	}
}

// TestModelBareCycleSendsModelChangedMsg: bare /model cycles through the
// model list and must notify the TUI the same way.
func TestModelBareCycleSendsModelChangedMsg(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "local"}, "test-model")
	app.tuiApp = tui.NewApp()
	app.initCommandsCore()

	out, _, err := app.cmdRegistry.Handle("/model")
	if err != nil {
		t.Fatalf("/model (bare) error = %v", err)
	}
	_ = out

	msg := receiveTUIMessage(t, app.tuiApp)
	if _, ok := msg.(tui.ModelChangedMsg); !ok {
		t.Fatalf("first TUI message = %T, want tui.ModelChangedMsg", msg)
	}
}
