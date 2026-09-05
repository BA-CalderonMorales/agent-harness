package main

import (
	"fmt"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/pkg/format"
)

// /diagnose answers "why does my harness behave the way it does" in one
// command: build identity, provider state, permission posture, and
// storage — everything the Logs tab scatters, gathered in one report.
// The provider readiness verdict is read from the TUI's boot probe; the
// command never re-probes the network.

// registerDiagnoseCommand wires /diagnose into the registry.
func registerDiagnoseCommand(app *App) {
	app.cmdRegistry.Register("diagnose", "Report provider, permissions, and storage state",
		func(args string) (string, error) {
			return app.diagnoseReport(), nil
		})
}

// diagnoseReport gathers the local facts. Lines, not prose — the
// output lands in the transcript like any command result.
func (app *App) diagnoseReport() string {
	var b strings.Builder

	b.WriteString("Harness\n")
	fmt.Fprintf(&b, "  version    %s\n", Version)

	b.WriteString("Provider\n")
	provider := app.config.Provider
	fmt.Fprintf(&b, "  provider   %s\n", provider)
	fmt.Fprintf(&b, "  endpoint   %s\n", app.config.EndpointURL)
	fmt.Fprintf(&b, "  model      %s\n", app.config.Model)
	if app.tuiApp != nil {
		readiness, notice := app.tuiApp.ProviderState()
		label := map[int]string{
			0: "checking", 1: "ready", 2: "warning",
			3: "unavailable", 4: "misconfigured",
		}[readiness]
		fmt.Fprintf(&b, "  state      %s\n", label)
		if notice != "" {
			fmt.Fprintf(&b, "  notice     %s\n", notice)
		}
	}

	b.WriteString("Permissions\n")
	fmt.Fprintf(&b, "  approval   %s\n", app.executionMode.String())
	fmt.Fprintf(&b, "  agent mode %s\n", string(app.agentMode))
	fmt.Fprintf(&b, "  read       %v · write %v · delete %v · execute %v\n",
		app.config.PermRead, app.config.PermWrite, app.config.PermDelete, app.config.PermExecute)
	if len(app.config.AlwaysAllow) > 0 {
		fmt.Fprintf(&b, "  always-on  %s\n", strings.Join(app.config.AlwaysAllow, ", "))
	}
	if len(app.config.AlwaysDeny) > 0 {
		fmt.Fprintf(&b, "  always-off %s\n", strings.Join(app.config.AlwaysDeny, ", "))
	}

	b.WriteString("Workspace\n")
	theme := app.config.Theme
	if theme == "" {
		theme = "default"
	}
	fmt.Fprintf(&b, "  theme      %s\n", theme)
	fmt.Fprintf(&b, "  persona    %s\n", app.config.Persona)

	b.WriteString("Storage\n")
	sessions := dirFootprint(config.DataSessions())
	audit := dirFootprint(config.DataAudit())
	logs := dirFootprint(config.DataLogs())
	toolResults := dirFootprint(config.DataToolResults())
	fmt.Fprintf(&b, "  sessions   %d files · %s\n", sessions.files, format.HumanBytes(sessions.bytes))
	fmt.Fprintf(&b, "  audit      %d files · %s\n", audit.files, format.HumanBytes(audit.bytes))
	fmt.Fprintf(&b, "  logs       %d files · %s\n", logs.files, format.HumanBytes(logs.bytes))
	fmt.Fprintf(&b, "  results    %d files · %s\n", toolResults.files, format.HumanBytes(toolResults.bytes))

	return strings.TrimRight(b.String(), "\n")
}
