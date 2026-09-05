package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/persona"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/commands"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
)

// newSelectorTestApp boots an App with the real command registry and a
// session to mutate — the selectors run exactly as a typed /command.
func newSelectorTestApp(t *testing.T) *App {
	t.Helper()
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "openrouter"}, "demo-1.0")
	app.initCommandsCore()
	app.initCommandsSystem()
	return app
}

// runSelector executes a slash command through the real registry.
func runSelector(app *App, input string) (string, error) {
	out, _, err := app.cmdRegistry.Handle(input)
	return out, err
}

// TestSelectorSemantics is the selector contract, one row per
// choice-like command: bare cycles to the next entry and wraps, an
// explicit sub-arg sets that entry, and a single-option cycle is a
// no-op with a notice — never an error.
func TestSelectorSemantics(t *testing.T) {
	app := newSelectorTestApp(t)

	// Wrap check: cycling len(list) times from the current value must
	// land back on it. Each row carries its selector, its value getter,
	// and the list it cycles.
	type selectorRow struct {
		name    string
		current func() string
		set     func(string)
		list    func() []string
	}
	rows := []selectorRow{
		{
			name:    "/effort",
			current: func() string { return app.config.Effort },
			set:     func(v string) { app.config.Effort = v },
			list:    func() []string { return config.EffortLevels },
		},
		{
			name:    "/mode",
			current: func() string { return string(app.agentMode) },
			set:     func(v string) { app.agentMode = agentMode(v) },
			list:    func() []string { return []string{"manual", "auto", "plan", "chat"} },
		},
		{
			name:    "/theme",
			current: func() string { return app.config.Theme },
			set:     func(v string) { app.config.Theme = v },
			list:    tui.ThemeNames,
		},
		{
			name:    "/persona",
			current: func() string { return app.session.Persona },
			set:     func(v string) { app.session.Persona = v },
			list: func() []string {
				names := make([]string, 0, len(persona.All()))
				for _, p := range persona.All() {
					names = append(names, p.String())
				}
				return names
			},
		},
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			// Named set first: an explicit sub-arg selects that entry.
			list := row.list()
			want := list[len(list)-1]
			if out, err := runSelector(app, fmt.Sprintf("%s %s", row.name, want)); err != nil {
				t.Fatalf("named set %q failed: %v (%s)", want, err, out)
			}
			if got := row.current(); got != want {
				t.Fatalf("named set landed on %q, want %q", got, want)
			}

			// Bare cycle: next entry, wrapping. One step from the last
			// entry must return to the first.
			if out, err := runSelector(app, row.name); err != nil {
				t.Fatalf("bare cycle failed: %v (%s)", err, out)
			}
			if got := row.current(); got != list[0] {
				t.Fatalf("bare cycle wrapped to %q, want %q", got, list[0])
			}

			// A full lap returns to the start: the cycle is a rotation.
			row.set(list[0])
			for range list {
				if _, err := runSelector(app, row.name); err != nil {
					t.Fatalf("full lap failed: %v", err)
				}
			}
			if got := row.current(); got != list[0] {
				t.Fatalf("full lap ended on %q, want %q", got, list[0])
			}
		})
	}
}

// TestSingleOptionSelectorIsNotice pins the single-entry contract at
// the shared seams: the list rotation returns the same entry, the model
// handler reports it as a notice (no error), and the settings cycle
// leaves the value alone.
func TestSingleOptionSelectorIsNotice(t *testing.T) {
	// The rotation helper: one entry cycles to itself.
	if got := commands.NextInList([]string{"solo"}, "solo"); got != "solo" {
		t.Fatalf("single-option rotation returned %q, want \"solo\"", got)
	}

	// The model handler: bare cycle with one model is a notice, not an
	// error — cycling is never a failure.
	set := ""
	out, err := commands.ModelHandler(
		func() string { return "solo" },
		func(m string) error { set = m; return nil },
		func() []string { return []string{"solo"} },
	)("")

	if err != nil {
		t.Fatalf("single-model cycle errored: %v", err)
	}
	if set != "" {
		t.Fatalf("single-model cycle changed the model to %q", set)
	}
	if !strings.Contains(out, "Only one model available") {
		t.Fatalf("single-model cycle notice missing: %q", out)
	}
}

// TestInvalidSelectorArgErrors: an explicit sub-arg that is not in the
// catalog must fail loudly — the notice contract protects cycles, not
// typos.
func TestInvalidSelectorArgErrors(t *testing.T) {
	app := newSelectorTestApp(t)
	for _, input := range []string{
		"/effort max",
		"/mode turbo",
		"/theme nonstop",
		"/persona wizard",
	} {
		if _, err := runSelector(app, input); err == nil {
			t.Fatalf("%s accepted an unknown entry", input)
		}
	}
}
