package main

import (
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/testharness/ttui"
)

// TestHelpOutputGolden pins every /help surface of the command set normal
// startup registers (newApp's initCommands, then run's TUI commands): the
// full listing, a known command, an unknown command, and an unknown slash
// command. A change to /help shows up as a golden diff.
func TestHelpOutputGolden(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "local"}, "test-model")
	app.initCommands()
	app.initCommandsForTUI(tui.NewApp())

	var b strings.Builder
	for _, input := range []string{"/help", "/help modes", "/help no-such-command", "/no-such-command"} {
		out, handled, err := app.cmdRegistry.Handle(input)
		b.WriteString("$ " + input + "\n")
		if err != nil {
			b.WriteString("error: " + err.Error() + "\n")
		}
		if !handled {
			b.WriteString("(not handled)\n")
		}
		b.WriteString(out + "\n\n")
	}
	// The golden must cover every registration group, not only core.
	for _, cmd := range []string{"/help ", "/pr ", "/branch ", "/doctor ", "/steer "} {
		if !strings.Contains(b.String(), cmd) {
			t.Fatalf("/help output is missing %s; the golden must use the startup command set", strings.TrimSpace(cmd))
		}
	}
	ttui.AssertGolden(t, "help-output", b.String())
}
