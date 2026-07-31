package main

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/atotto/clipboard"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/persona"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/state"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// tuiHomeDelegate connects TUI home dashboard to the app.
type tuiHomeDelegate struct {
	app    *App
	tuiApp *tui.App
}

func (d *tuiHomeDelegate) OnNewChat() {
	if d.app.session == nil {
		d.app.session = d.app.sessionManager.CreateSession("")
	}
	if d.app.session != nil && len(d.app.session.Messages) > 0 {
		if _, err := d.app.sessionManager.SaveCurrent(); err != nil {
			d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Failed to save session: %v", err), Type: "error"})
			return
		}
	}
	model := d.app.session.Model
	personaName := d.app.session.Persona
	d.app.session = d.app.sessionManager.CreateSession(model)
	d.app.session.Persona = personaName
	d.app.sessionManager.SetCurrent(d.app.session)
	if _, err := d.app.sessionManager.SaveCurrent(); err != nil {
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Failed to persist new session: %v", err), Type: "error"})
		return
	}
	d.tuiApp.Send(tui.SessionActivatedMsg{
		SessionID:      d.app.session.ID,
		Transcript:     nil,
		Model:          d.app.session.Model,
		Persona:        d.app.session.Persona,
		Sessions:       d.app.getSessionInfos(),
		Notice:         sprintf("Started new chat %s", d.app.session.ID[:8]),
		NoticeType:     "success",
		SwitchToChat:   true,
		PermissionMode: d.app.config.PermissionMode.String(),
		EstTokens:      d.app.session.EstimateTokens(),
	})
}

func (d *tuiHomeDelegate) OnExportSession() {
	path, err := exportSession(d.app.session, "")
	if err != nil {
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Export failed: %v", err), Type: "error"})
		return
	}
	absPath, absErr := filepath.Abs(path)
	if absErr != nil {
		absPath = path
	}
	d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Exported to %s", absPath), Type: "success"})
}

func (d *tuiHomeDelegate) OnLoadSession(id string) {
	if d.app.session != nil && d.app.session.ID != id {
		if _, err := d.app.sessionManager.SaveCurrent(); err != nil {
			d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Failed to save session: %v", err), Type: "error"})
			return
		}
	}
	session, err := d.app.sessionManager.LoadSession(id)
	if err != nil {
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Failed to load session: %v", err), Type: "error"})
		return
	}
	d.app.session = session
	d.app.costTracker.SetModel(session.Model)
	d.tuiApp.Send(tui.SessionActivatedMsg{
		SessionID:      session.ID,
		Transcript:     session.Messages,
		Model:          session.Model,
		Persona:        session.Persona,
		Sessions:       d.app.getSessionInfos(),
		Notice:         sprintf("Loaded session %s", shortID(id)),
		NoticeType:     "success",
		SwitchToChat:   true,
		PermissionMode: d.app.config.PermissionMode.String(),
		EstTokens:      session.EstimateTokens(),
	})
}

// tuiSessionsDelegate connects TUI sessions to the app.
type tuiSessionsDelegate struct {
	app    *App
	tuiApp *tui.App
}

func (d *tuiSessionsDelegate) OnSessionSelect(id string) {
	if d.app.session != nil && d.app.session.ID != id {
		if _, err := d.app.sessionManager.SaveCurrent(); err != nil {
			d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Failed to save session: %v", err), Type: "error"})
			return
		}
	}
	session, err := d.app.sessionManager.LoadSession(id)
	if err != nil {
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Failed to load session: %v", err), Type: "error"})
		return
	}
	d.app.session = session
	d.app.costTracker.SetModel(session.Model)
	d.tuiApp.Send(tui.SessionActivatedMsg{
		SessionID:      session.ID,
		Transcript:     session.Messages,
		Model:          session.Model,
		Persona:        session.Persona,
		Sessions:       d.app.getSessionInfos(),
		Notice:         sprintf("Loaded session %s", shortID(id)),
		NoticeType:     "success",
		SwitchToChat:   true,
		PermissionMode: d.app.config.PermissionMode.String(),
		EstTokens:      session.EstimateTokens(),
	})
}

func (d *tuiSessionsDelegate) OnSessionDelete(id string) {
	isActive := d.app.session != nil && d.app.session.ID == id
	if isActive {
		model := d.app.session.Model
		personaName := d.app.session.Persona
		replacement := d.app.sessionManager.CreateSession(model)
		replacement.Persona = personaName
		d.app.sessionManager.SetCurrent(replacement)
		d.app.session = replacement
		if _, err := d.app.sessionManager.SaveCurrent(); err != nil {
			d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Failed to create replacement session: %v", err), Type: "error"})
			replacement.ID = ""
			d.app.session = nil
			return
		}
	}
	if err := d.app.sessionManager.DeleteSession(id); err != nil {
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Failed to delete session: %v", err), Type: "error"})
		return
	}
	if isActive {
		d.tuiApp.Send(tui.SessionActivatedMsg{
			SessionID:      d.app.session.ID,
			Transcript:     nil,
			Model:          d.app.session.Model,
			Persona:        d.app.session.Persona,
			Sessions:       d.app.getSessionInfos(),
			Notice:         sprintf("Deleted session %s; replacement active", shortID(id)),
			NoticeType:     "success",
			SwitchToChat:   true,
			PermissionMode: d.app.config.PermissionMode.String(),
			EstTokens:      d.app.session.EstimateTokens(),
		})
	} else {
		d.tuiApp.Send(tui.SessionsRefreshedMsg{
			Sessions:   d.app.getSessionInfos(),
			Notice:     sprintf("Deleted session %s", shortID(id)),
			NoticeType: "success",
		})
	}
}

func (d *tuiSessionsDelegate) OnSessionExport(id string) {
	session, err := d.app.sessionManager.ReadSession(id)
	if err != nil {
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Failed to load session for export: %v", err), Type: "error"})
		return
	}
	path, err := exportSession(session, "")
	if err != nil {
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Failed to export: %v", err), Type: "error"})
		return
	}
	absPath, absErr := filepath.Abs(path)
	if absErr != nil {
		absPath = path
	}
	d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Exported to %s", absPath), Type: "success"})
}

func (d *tuiSessionsDelegate) OnSessionCopy(id string) {
	session, err := d.app.sessionManager.ReadSession(id)
	if err != nil {
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Failed to load session for copy: %v", err), Type: "error"})
		return
	}

	content := formatSessionForClipboard(session)
	if err := clipboard.WriteAll(content); err != nil {
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Failed to copy to clipboard: %v", err), Type: "error"})
		return
	}

	d.tuiApp.Send(tui.StatusMsg{
		Text: sprintf("Copied %d messages from session %s", len(session.Messages), shortID(id)),
		Type: "success",
	})
}

func (d *tuiSessionsDelegate) OnSessionLoad() {
	d.tuiApp.Send(tui.SessionsRefreshedMsg{
		Sessions: d.app.getSessionInfos(),
	})
}

func (d *tuiSessionsDelegate) OnSessionNew() {
	if d.app.session == nil {
		d.app.session = d.app.sessionManager.CreateSession("")
	}
	if d.app.session != nil && len(d.app.session.Messages) > 0 {
		if _, err := d.app.sessionManager.SaveCurrent(); err != nil {
			d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Failed to save session: %v", err), Type: "error"})
			return
		}
	}
	model := d.app.session.Model
	personaName := d.app.session.Persona
	d.app.session = d.app.sessionManager.CreateSession(model)
	d.app.session.Persona = personaName
	d.app.sessionManager.SetCurrent(d.app.session)
	if _, err := d.app.sessionManager.SaveCurrent(); err != nil {
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Failed to persist new session: %v", err), Type: "error"})
		return
	}
	d.tuiApp.Send(tui.SessionActivatedMsg{
		SessionID:      d.app.session.ID,
		Transcript:     nil,
		Model:          d.app.session.Model,
		Persona:        d.app.session.Persona,
		Sessions:       d.app.getSessionInfos(),
		Notice:         sprintf("Started new chat %s", d.app.session.ID[:8]),
		NoticeType:     "success",
		SwitchToChat:   true,
		PermissionMode: d.app.config.PermissionMode.String(),
		EstTokens:      d.app.session.EstimateTokens(),
	})
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
		d.rebuildLLMClient()
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Provider updated to: %s", value), Type: "success"})
	case "runtime":
		d.app.config.Runtime = value
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Runtime updated to: %s", value), Type: "success"})
	case "endpoint_url":
		d.app.config.EndpointURL = value
		d.rebuildLLMClient()
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Endpoint updated to: %s", value), Type: "success"})
	case "context_length":
		if n, err := strconv.Atoi(value); err == nil && n > 0 {
			d.app.config.ContextLength = n
			if d.app.loop != nil {
				d.app.loop.Config.BlockingTokenLimit = n
			}
			d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Context length updated to: %d", n), Type: "success"})
		}
	case "max_tokens":
		if n, err := strconv.Atoi(value); err == nil && n > 0 {
			d.app.config.MaxTokens = n
			d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Max tokens updated to: %d", n), Type: "success"})
		}
	case "temperature":
		if n, err := strconv.ParseFloat(value, 64); err == nil {
			d.app.config.Temperature = n
			d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Temperature updated to: %.2f", n), Type: "success"})
		}
	case "permissions":
		d.handlePermissionModeChange(value)
	case "execution_mode":
		d.handleExecutionModeChange(value)
	case "perm_read":
		d.app.config.PermRead = value == "true"
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Read permission: %s", boolToEnabled(d.app.config.PermRead)), Type: "info"})
	case "perm_write":
		d.app.config.PermWrite = value == "true"
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Write permission: %s", boolToEnabled(d.app.config.PermWrite)), Type: "info"})
	case "perm_delete":
		d.app.config.PermDelete = value == "true"
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Delete permission: %s", boolToEnabled(d.app.config.PermDelete)), Type: "info"})
	case "perm_execute":
		d.app.config.PermExecute = value == "true"
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Execute permission: %s", boolToEnabled(d.app.config.PermExecute)), Type: "info"})
	case "session_dir":
		d.app.config.SessionDir = value
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Session directory: %s (applied on next restart)", value), Type: "success"})
	}
	d.tuiApp.Send(tui.SessionsRefreshedMsg{
		Sessions: d.app.getSessionInfos(),
	})
}

func (d *tuiSettingsDelegate) rebuildLLMClient() {
	d.app.rebuildLLMClient()
}

// refreshPersonaUI updates persona-dependent UI state after a persona change.
func (d *tuiSettingsDelegate) refreshPersonaUI(persona string) {
	d.tuiApp.Send(tui.SessionActivatedMsg{
		SessionID:      d.app.session.ID,
		Model:          d.app.session.Model,
		Persona:        persona,
		Sessions:       d.app.getSessionInfos(),
		PermissionMode: d.app.config.PermissionMode.String(),
		EstTokens:      d.app.session.EstimateTokens(),
	})
}

// handlePersonaChange updates the persona and refreshes the UI.
func (d *tuiSettingsDelegate) handlePersonaChange(value string) {
	if p, err := persona.Parse(value); err == nil {
		d.app.session.Persona = p.String()
		d.refreshPersonaUI(p.String())
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Persona: %s — %s", p.DisplayName(), p.Description()), Type: "success"})
	} else {
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Invalid persona: %v", err), Type: "error"})
	}
}

// handleModelChange updates the model and saves to config.
func (d *tuiSettingsDelegate) handleModelChange(value string) {
	d.app.session.Model = value
	d.app.costTracker.SetModel(value)
	d.tuiApp.Send(tui.ModelChangedMsg{Model: value})

	credManager := config.NewCredentialManager()
	if err := credManager.UpdateDefaultModel(value); err != nil {
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Warning: failed to save default model: %v", err), Type: "warning"})
	} else {
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Default model: %s", value), Type: "success"})
	}
}

// handlePermissionModeChange updates permission mode and syncs granular toggles.
func (d *tuiSettingsDelegate) handlePermissionModeChange(value string) {
	if mode, err := config.ParsePermissionMode(value); err == nil {
		d.app.config.PermissionMode = mode
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
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Permission mode: %s", mode.String()), Type: "success"})
	} else {
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Invalid permission mode: %v", err), Type: "error"})
	}
}

// handleExecutionModeChange updates execution mode.
func (d *tuiSettingsDelegate) handleExecutionModeChange(value string) {
	if mode, err := approval.ParseExecutionMode(value); err == nil {
		d.app.executionMode = mode
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Execution mode: %s", mode.String()), Type: "success"})
	} else {
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Invalid execution mode: %v", err), Type: "error"})
	}
}

// OnSettingReset handles reset request.
func (d *tuiSettingsDelegate) OnSettingReset() {
	d.tuiApp.Send(tui.StatusMsg{Text: "Reset to defaults not implemented", Type: "warning"})
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

func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}
