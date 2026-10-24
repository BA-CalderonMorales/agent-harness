package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
)

// The first-run checklist.
//
// The status bar tells a first-run user that something is missing
// ("[! setup required] (l: login)"). /doctor answers the next question:
// what exactly is wrong, what is already fine, and which key fixes each
// gap. Every line is one fact with one fix, so nothing here needs a
// paragraph.

// readinessLine is one row of the checklist.
type readinessLine struct {
	status string
	label  string
	detail string
	fix    string
}

// formatReadinessReport renders the checklist.
func (app *App) formatReadinessReport() string {
	lines := app.readinessLines()

	var sb strings.Builder
	sb.WriteString("Readiness\n\n")
	for _, line := range lines {
		sb.WriteString(sprintf("  %-5s %-12s %s\n", line.status, line.label, line.detail))
	}

	next := firstFix(lines)
	if next != "" {
		sb.WriteString("\nNext: ")
		sb.WriteString(next)
	} else {
		sb.WriteString("\nEverything checks out.")
	}
	return sb.String()
}

// firstFix returns the first fix worth acting on.
func firstFix(lines []readinessLine) string {
	for _, line := range lines {
		if line.fix != "" {
			return line.fix
		}
	}
	return ""
}

// readinessLines gathers the checklist facts.
func (app *App) readinessLines() []readinessLine {
	lines := make([]readinessLine, 0, 8)

	workspace := app.cwd
	if workspace == "" {
		workspace = "(unknown)"
	}
	detail := workspace
	if app.gitContext != nil && app.gitContext.IsRepo {
		detail = sprintf("%s (git: %s)", workspace, app.gitContext.Branch)
	}
	lines = append(lines, readinessLine{status: "ok", label: "workspace", detail: detail})

	provider := app.config.Provider
	if provider == "" {
		provider = "local"
	}
	lines = append(lines, readinessLine{status: "ok", label: "provider", detail: provider})

	key := ""
	if app.config.APIKey != "" {
		key = "set"
	}
	if key == "" {
		lines = append(lines, readinessLine{
			status: "warn",
			label:  "api key",
			detail: "none for " + provider,
			fix:    sprintf("press l to log in, or set AH_API_KEY for %s", provider),
		})
	} else {
		lines = append(lines, readinessLine{status: "ok", label: "api key", detail: "set"})
	}

	readiness, message := app.probeProvider()
	switch readiness {
	case llm.ProviderReady:
		lines = append(lines, readinessLine{status: "ok", label: "endpoint", detail: message})
	case llm.ProviderWarning:
		lines = append(lines, readinessLine{status: "warn", label: "endpoint", detail: message})
	default:
		lines = append(lines, readinessLine{
			status: "fail",
			label:  "endpoint",
			detail: message,
			fix:    "check AH_ENDPOINT_URL and your network, then run /diagnose",
		})
	}

	model := app.session.Model
	if model == "" {
		model = "(none selected)"
	}
	lines = append(lines, readinessLine{status: "ok", label: "model", detail: model})

	skills := app.loadSkills().All()
	skillDetail := fmt.Sprintf("%d available (/skills)", len(skills))
	lines = append(lines, readinessLine{status: "ok", label: "skills", detail: skillDetail})

	agents := app.loadAgentDefs().All()
	agentDetail := fmt.Sprintf("%d available (/agents)", len(agents))
	lines = append(lines, readinessLine{status: "ok", label: "sub-agents", detail: agentDetail})

	mcpTools := 0
	if app.mcpManager != nil {
		mcpTools = len(app.mcpManager.AllToolDefs())
	}
	lines = append(lines, readinessLine{
		status: "ok",
		label:  "mcp",
		detail: fmt.Sprintf("%d tools", mcpTools),
	})

	mode := app.config.PermissionMode.String()
	if mode == "" {
		mode = "interactive"
	}
	lines = append(lines, readinessLine{status: "ok", label: "permissions", detail: mode})

	return lines
}

// probeProvider checks the configured endpoint with the same prober the
// boot check uses, so /doctor and the status bar agree.
func (app *App) probeProvider() (llm.ProviderReadiness, string) {
	provider := app.config.Provider
	if provider == "" {
		provider = "local"
	}
	prober := llm.NewHTTPProber(provider, app.config.APIKey, app.config.EndpointURL)
	return prober.Probe(context.Background())
}
