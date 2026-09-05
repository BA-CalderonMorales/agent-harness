package main

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/persona"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"slices"
	"strconv"
)

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
		d.app.config.EndpointURL = config.DefaultEndpointForProvider(value)
		d.app.config.ApplyTimeoutDefaults()
		d.rebuildLLMClient()
		d.app.commitConfigChange()
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Provider updated to: %s", value), Type: "success"})
	case "runtime":
		d.app.config.Runtime = value
		d.app.commitConfigChange()
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Runtime updated to: %s", value), Type: "success"})
	case "endpoint_url":
		d.app.config.EndpointURL = value
		d.rebuildLLMClient()
		d.app.commitConfigChange()
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Endpoint updated to: %s", value), Type: "success"})
	case "context_length":
		if n, err := strconv.Atoi(value); err == nil && n > 0 {
			d.app.config.ContextLength = n
			if d.app.loop != nil {
				d.app.loop.Config.BlockingTokenLimit = n
			}
			d.app.commitConfigChange()
			d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Context length updated to: %d", n), Type: "success"})
		}
	case "max_tokens":
		if n, err := strconv.Atoi(value); err == nil && n > 0 {
			d.app.config.MaxTokens = n
			d.app.commitConfigChange()
			d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Max tokens updated to: %d", n), Type: "success"})
		}
	case "temperature":
		if n, err := strconv.ParseFloat(value, 64); err == nil && n >= 0 && n <= 2 {
			d.app.config.Temperature = n
			d.app.commitConfigChange()
			d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Temperature updated to: %.2f", n), Type: "success"})
		} else {
			// The value was rejected: say so instead of silently keeping
			// the old temperature while the row shows the rejected text.
			d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Temperature must be a number between 0.0 and 2.0; got %q.", value), Type: "error"})
		}
	case "reasoning_effort":
		if slices.Contains(config.EffortLevels, value) {
			d.app.config.Effort = value
			d.app.commitConfigChange()
			d.app.rebuildLLMClient()
			d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Reasoning effort: %s", value), Type: "success"})
		}
	case "permissions":
		d.handlePermissionModeChange(value)
	case "theme":
		if d.tuiApp.ApplyTheme(value) {
			theme, _ := tui.LookupTheme(value)
			d.app.config.Theme = theme.Name
			d.app.commitConfigChange()
			d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Theme: %s", theme.Name), Type: "success"})
		} else {
			d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Unknown theme %q; try /theme for the catalog.", value), Type: "error"})
		}
	case "execution_mode":
		d.handleExecutionModeChange(value)
	case "perm_read":
		d.app.config.PermRead = value == "true"
		d.app.config.PermExplicit = true
		d.app.commitConfigChange()
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Read permission: %s", boolToEnabled(d.app.config.PermRead)), Type: "info"})
	case "perm_write":
		d.app.config.PermWrite = value == "true"
		d.app.config.PermExplicit = true
		d.app.commitConfigChange()
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Write permission: %s", boolToEnabled(d.app.config.PermWrite)), Type: "info"})
	case "perm_delete":
		d.app.config.PermDelete = value == "true"
		d.app.config.PermExplicit = true
		d.app.commitConfigChange()
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Delete permission: %s", boolToEnabled(d.app.config.PermDelete)), Type: "info"})
	case "perm_execute":
		d.app.config.PermExecute = value == "true"
		d.app.config.PermExplicit = true
		d.app.commitConfigChange()
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

// handleModelChange updates the model and saves it as the default.
func (d *tuiSettingsDelegate) handleModelChange(value string) {
	d.app.session.Model = value
	d.app.costTracker.SetModel(value)
	d.tuiApp.Send(tui.ModelChangedMsg{Model: value})
	d.app.commitConfigChange()

	// Keep the encrypted store in sync only when it exists; model now
	// persists via ~/.agent-harness/settings.json so this is optional.
	credManager := config.NewCredentialManager()
	if credManager.HasSecureCredentials() {
		if err := credManager.UpdateDefaultModel(value); err != nil {
			d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Warning: failed to save default model: %v", err), Type: "warning"})
			return
		}
	}
	d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Default model: %s", value), Type: "success"})
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
		// Choosing a preset re-owns the granular toggles: an explicit
		// toggle from before the switch must not override the preset
		// the user just picked.
		d.app.config.PermExplicit = false
		d.app.commitConfigChange()
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
