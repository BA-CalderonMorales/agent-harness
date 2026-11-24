package main

import (
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/testharness/ttui"
)

// TestHelpOutputGolden pins every /help surface of the real command set:
// the full listing, a known command, an unknown command, and an unknown
// slash command. A change to /help shows up as a golden diff.
func TestHelpOutputGolden(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "local"}, "test-model")
	app.initCommandsCore()

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
	ttui.AssertGolden(t, "help-output", b.String())
}
