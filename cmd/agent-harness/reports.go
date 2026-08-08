package main

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/core/audit"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/persona"
	"strings"
)

// getMemoryInfo formats system prompt and session memory context.
func (app *App) getMemoryInfo() string {
	var sb strings.Builder
	sb.WriteString("Memory & Context State\n")
	sb.WriteString(sprintf("  Session ID:      %s\n", app.session.ID))
	sb.WriteString(sprintf("  Model:           %s\n", app.session.Model))
	sb.WriteString(sprintf("  Persona:         %s\n", app.session.Persona))
	sb.WriteString(sprintf("  Message Count:   %d\n", len(app.session.Messages)))
	sb.WriteString(sprintf("  Workspace:       %s\n", app.cwd))
	sb.WriteString(sprintf("  Plan Mode:       %v\n", app.planMode))
	sb.WriteString("\nSystem Prompt Snippet:\n")
	prompt := app.buildSystemPrompt()
	if len(prompt) > 300 {
		sb.WriteString(prompt[:300] + "...\n")
	} else {
		sb.WriteString(prompt + "\n")
	}
	return sb.String()
}

// formatConfig formats app configuration.
func (app *App) formatConfig() string {
	var sb strings.Builder
	sb.WriteString("Configuration\n")
	sb.WriteString(sprintf("  Provider:        %s\n", app.config.Provider))
	sb.WriteString(sprintf("  Model:           %s\n", app.config.Model))
	sb.WriteString(sprintf("  Runtime:         %s\n", app.config.Runtime))
	sb.WriteString(sprintf("  Endpoint URL:    %s\n", app.config.EndpointURL))
	sb.WriteString(sprintf("  Context Length:  %d\n", app.config.ContextLength))
	sb.WriteString(sprintf("  Max Tokens:      %d\n", app.config.MaxTokens))
	sb.WriteString(sprintf("  Temperature:     %.2f\n", app.config.Temperature))
	sb.WriteString(sprintf("  Workspace Path:  %s\n", app.cwd))
	sb.WriteString(sprintf("  Permission Mode: %s\n", app.config.PermissionMode))
	sb.WriteString(sprintf("  Execution Mode:  %s\n", app.executionMode))
	return sb.String()
}

// updateConfiguration updates configuration options dynamically via /config and re-probes.

// getPermissionsReport formats active permissions and modes.
func (app *App) getPermissionsReport() string {
	var sb strings.Builder
	sb.WriteString("Permissions & Mode State\n")
	sb.WriteString(sprintf("  Permission Mode: %s\n", app.config.PermissionMode))
	sb.WriteString(sprintf("  Execution Mode:  %s\n", app.executionMode))
	sb.WriteString(sprintf("  Allow Read:      %v\n", app.config.PermRead))
	sb.WriteString(sprintf("  Allow Write:     %v\n", app.config.PermWrite))
	sb.WriteString(sprintf("  Allow Delete:    %v\n", app.config.PermDelete))
	sb.WriteString(sprintf("  Allow Execute:   %v\n", app.config.PermExecute))
	sb.WriteString("\nAvailable Execution Modes:\n")
	sb.WriteString("  interactive      Ask before executing commands\n")
	sb.WriteString("  yolo             Execute all permitted commands automatically\n")
	sb.WriteString("\nRun /permissions <interactive|yolo> to change mode.")
	return sb.String()
}

// formatAgentsList formats sub-agent personas.
func (app *App) formatAgentsList() string {
	var sb strings.Builder
	sb.WriteString("Available Agent Personas:\n")
	for _, p := range persona.All() {
		sb.WriteString(sprintf("  %-12s %s\n", p.String(), p.Description()))
	}
	sb.WriteString("\nSub-agent Delegation:\n")
	sb.WriteString("  Agent-harness spawns sub-agents dynamically for complex multi-step workflows.")
	return sb.String()
}

// getAuditLog returns formatted recent tool execution log.
func (app *App) getAuditLog() string {
	logger, err := audit.NewLogger()
	if err != nil {
		return sprintf("Audit log error: %v", err)
	}
	entries, err := logger.Recent(20)
	if err != nil {
		return sprintf("Failed to load audit entries: %v", err)
	}
	if len(entries) == 0 {
		return "No recent audit log entries."
	}
	var sb strings.Builder
	sb.WriteString("Recent Audit Log Entries:\n")
	for _, e := range entries {
		sb.WriteString(sprintf("  [%s] %-12s %-10s approved=%v (%dms)\n",
			e.Timestamp.Format("15:04:05"), e.ToolName, e.Decision, e.Approved, e.DurationMillis))
	}
	return sb.String()
}
