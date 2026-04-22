package main

import (
	"os"
	"strings"

	"github.com/atotto/clipboard"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/persona"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/state"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// tuiHomeDelegate connects TUI home dashboard to the app.
type tuiHomeDelegate struct {
	app    *App
	tuiApp *tui.App
}

func (d *tuiHomeDelegate) OnNewChat() {
	d.app.session = d.app.session.Clear()
	d.tuiApp.Send(tui.ClearChatMsg{FollowUpMsg: "Starting fresh conversation."})
}

func (d *tuiHomeDelegate) OnRunTests() {
	result, err := d.app.runTests()
	if err != nil {
		d.tuiApp.AddMessage("system", sprintf("Tests failed: %v", err))
		return
	}
	d.tuiApp.AddMessage("system", result)
}

func (d *tuiHomeDelegate) OnExportSession() {
	path := sprintf("session-%s.md", d.app.session.ID[:8])
	md := d.app.session.ExportToMarkdown()
	if err := os.WriteFile(path, []byte(md), 0644); err != nil {
		d.tuiApp.AddMessage("system", sprintf("Export failed: %v", err))
		return
	}
	d.tuiApp.AddMessage("system", sprintf("Exported to %s", path))
}

func (d *tuiHomeDelegate) OnSwitchPersona() {
	d.tuiApp.AddMessage("system", "Use /persona <name> to switch. Available: developer, designer, pm, scientist, explorer")
}

func (d *tuiHomeDelegate) OnLoadSession(id string) {
	session, err := d.app.sessionManager.LoadSession(id)
	if err != nil {
		d.tuiApp.AddMessage("system", sprintf("Failed to load session: %v", err))
		return
	}
	d.app.session = session
	d.tuiApp.SetChatPersona(session.Persona)
	d.tuiApp.AddMessage("system", sprintf("Loaded session %s", id[:8]))
	d.tuiApp.RefreshSessions(d.app.getSessionInfos())
}

// tuiSessionsDelegate connects TUI sessions to the app.
type tuiSessionsDelegate struct {
	app    *App
	tuiApp *tui.App
}

// OnSessionSelect handles session selection.
func (d *tuiSessionsDelegate) OnSessionSelect(id string) {
	session, err := d.app.sessionManager.LoadSession(id)
	if err != nil {
		d.tuiApp.AddMessage("system", sprintf("Failed to load session: %v", err))
		return
	}
	d.app.session = session
	d.tuiApp.SetChatPersona(session.Persona)
	d.tuiApp.AddMessage("system", sprintf("Loaded session %s", id[:8]))
	d.tuiApp.RefreshSessions(d.app.getSessionInfos())
}

// OnSessionDelete handles session deletion.
func (d *tuiSessionsDelegate) OnSessionDelete(id string) {
	if err := d.app.sessionManager.DeleteSession(id); err != nil {
		d.tuiApp.AddMessage("system", sprintf("Failed to delete session: %v", err))
		return
	}
	d.tuiApp.AddMessage("system", sprintf("Deleted session %s", id[:8]))
	d.tuiApp.RefreshSessions(d.app.getSessionInfos())
}

// OnSessionExport handles session export.
func (d *tuiSessionsDelegate) OnSessionExport(id string) {
	path := sprintf("session-%s.json", id[:8])
	if err := d.app.session.SaveToFile(path); err != nil {
		d.tuiApp.AddMessage("system", sprintf("Failed to export: %v", err))
		return
	}
	d.tuiApp.AddMessage("system", sprintf("Exported to %s", path))
}

// OnSessionCopy copies session content to clipboard.
func (d *tuiSessionsDelegate) OnSessionCopy(id string) {
	session, err := d.app.sessionManager.LoadSession(id)
	if err != nil {
		d.tuiApp.AddMessage("system", sprintf("Failed to load session for copy: %v", err))
		return
	}

	content := formatSessionForClipboard(session)
	if err := clipboard.WriteAll(content); err != nil {
		d.tuiApp.AddMessage("system", sprintf("Failed to copy to clipboard: %v", err))
		return
	}

	d.tuiApp.AddMessage("system", sprintf("Copied conversation from session %s to clipboard (%d messages)", id[:8], len(session.Messages)))
}

// OnSessionLoad triggers session list refresh.
func (d *tuiSessionsDelegate) OnSessionLoad() {
	d.tuiApp.RefreshSessions(d.app.getSessionInfos())
}

// formatSessionForClipboard formats a session for clipboard copy.
func formatSessionForClipboard(session *state.Session) string {
	var b strings.Builder
	b.WriteString(sprintf("Session: %s\n", session.ID[:8]))
	b.WriteString(sprintf("Model: %s\n", session.Model))
	b.WriteString(sprintf("Created: %s\n\n", session.CreatedAt.Format("2006-01-02 15:04")))
	b.WriteString("=== Conversation ===\n\n")

	for _, msg := range session.Messages {
		b.WriteString(formatMessageForClipboard(msg))
		b.WriteString("\n")
	}
	return b.String()
}

// formatMessageForClipboard formats a single message for clipboard.
func formatMessageForClipboard(msg types.Message) string {
	var b strings.Builder

	switch msg.Role {
	case types.RoleUser:
		b.WriteString("User:\n")
	case types.RoleAssistant:
		b.WriteString("Assistant:\n")
	case types.RoleSystem:
		b.WriteString("System:\n")
	default:
		b.WriteString(sprintf("%s:\n", msg.Role))
	}

	for _, block := range msg.Content {
		if textBlock, ok := block.(types.TextBlock); ok {
			b.WriteString(textBlock.Text)
			b.WriteString("\n")
		}
	}
	return b.String()
}

// tuiSettingsDelegate connects TUI settings to the app.
type tuiSettingsDelegate struct {
	app    *App
	tuiApp *tui.App
}

// OnSettingChange handles setting changes.
func (d *tuiSettingsDelegate) OnSettingChange(key, value string) {
	switch key {
	case "persona":
		d.handlePersonaChange(value)
	case "model":
		d.handleModelChange(value)
	case "provider":
		d.app.config.Provider = value
		// Recreate LLM client with new provider base URL
		d.app.client = llm.NewHTTPClient(d.app.config.Provider, d.app.config.APIKey)
		// Refresh model list for new provider
		d.tuiApp.SetModels(d.app.getModelItems())
		d.tuiApp.AddMessage("system", sprintf("Provider updated to: %s", value))
	case "permissions":
		d.handlePermissionModeChange(value)
	case "execution_mode":
		d.handleExecutionModeChange(value)
	case "perm_read":
		d.app.config.PermRead = value == "true"
		d.tuiApp.AddMessage("system", sprintf("Read permission: %s", boolToEnabled(d.app.config.PermRead)))
	case "perm_write":
		d.app.config.PermWrite = value == "true"
		d.tuiApp.AddMessage("system", sprintf("Write permission: %s", boolToEnabled(d.app.config.PermWrite)))
	case "perm_delete":
		d.app.config.PermDelete = value == "true"
		d.tuiApp.AddMessage("system", sprintf("Delete permission: %s", boolToEnabled(d.app.config.PermDelete)))
	case "perm_execute":
		d.app.config.PermExecute = value == "true"
		d.tuiApp.AddMessage("system", sprintf("Execute permission: %s", boolToEnabled(d.app.config.PermExecute)))
	}
	d.tuiApp.SetSettings(d.app.getSettings())
}

// refreshPersonaUI updates persona-dependent UI state after a persona change.
func (d *tuiSettingsDelegate) refreshPersonaUI(persona string) {
	d.tuiApp.SetChatPersona(persona)
	d.tuiApp.SetSettings(d.app.getSettings())
	d.tuiApp.SetHomeStatus(d.app.session.Model, d.app.config.PermissionMode.String(), persona, d.app.session.EstimateTokens())
}

// handlePersonaChange updates the persona and refreshes the UI.
func (d *tuiSettingsDelegate) handlePersonaChange(value string) {
	if p, err := persona.Parse(value); err == nil {
		d.app.session.Persona = p.String()
		d.refreshPersonaUI(p.String())
		d.tuiApp.AddMessage("system", sprintf("Persona switched to: %s — %s", p.DisplayName(), p.Description()))
	} else {
		d.tuiApp.AddMessage("system", sprintf("Invalid persona: %v", err))
	}
}

// handleModelChange updates the model and saves to config.
func (d *tuiSettingsDelegate) handleModelChange(value string) {
	d.app.session.Model = value
	d.app.costTracker.SetModel(value)
	d.tuiApp.Send(tui.ModelChangedMsg{Model: value})

	credManager := config.NewCredentialManager()
	if err := credManager.UpdateDefaultModel(value); err != nil {
		d.tuiApp.AddMessage("system", sprintf("Warning: failed to save default model: %v", err))
	} else {
		d.tuiApp.AddMessage("system", sprintf("Default model updated to: %s", value))
	}
}

// handlePermissionModeChange updates permission mode and syncs granular toggles.
func (d *tuiSettingsDelegate) handlePermissionModeChange(value string) {
	if mode, err := config.ParsePermissionMode(value); err == nil {
		d.app.config.PermissionMode = mode
		// Sync granular toggles to match the preset
		switch mode {
		case config.PermissionReadOnly:
			d.app.config.PermRead = true
			d.app.config.PermWrite = false
			d.app.config.PermDelete = false
			d.app.config.PermExecute = false
		case config.PermissionWorkspaceWrite:
			d.app.config.PermRead = true
			d.app.config.PermWrite = true
			d.app.config.PermDelete = false
			d.app.config.PermExecute = false
		case config.PermissionDangerFullAccess:
			d.app.config.PermRead = true
			d.app.config.PermWrite = true
			d.app.config.PermDelete = true
			d.app.config.PermExecute = true
		}
		d.tuiApp.AddMessage("system", sprintf("Permission mode: %s", mode.String()))
	}
}

// handleExecutionModeChange updates execution mode.
func (d *tuiSettingsDelegate) handleExecutionModeChange(value string) {
	if mode, err := approval.ParseExecutionMode(value); err == nil {
		d.app.executionMode = mode
		d.tuiApp.AddMessage("system", sprintf("Execution mode set to: %s", mode.String()))
	}
}

// OnSettingReset handles reset request.
func (d *tuiSettingsDelegate) OnSettingReset() {
	d.tuiApp.AddMessage("system", "Reset to defaults not implemented")
}

// OnSettingReload handles reload request.
func (d *tuiSettingsDelegate) OnSettingReload() {
	d.tuiApp.SetSettings(d.app.getSettings())
}

// boolToEnabled converts bool to enabled/disabled string.
func boolToEnabled(b bool) string {
	if b {
		return "enabled"
	}
	return "disabled"
}
