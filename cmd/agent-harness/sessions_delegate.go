package main

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/core/state"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"github.com/atotto/clipboard"
	"path/filepath"
	"strings"
)

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
	d.app.deleteSession(id, d.tuiApp)
}

// deleteSession removes a session, replacing the active one if needed so
// the harness always has a live session. Shared by the Sessions tab and
// the Home dashboard's Recent Sessions list.
func (app *App) deleteSession(id string, tuiApp *tui.App) {
	isActive := app.session != nil && app.session.ID == id
	if isActive {
		model := app.session.Model
		personaName := app.session.Persona
		replacement := app.sessionManager.CreateSession(model)
		replacement.Persona = personaName
		app.sessionManager.SetCurrent(replacement)
		app.session = replacement
		if _, err := app.sessionManager.SaveCurrent(); err != nil {
			tuiApp.Send(tui.StatusMsg{Text: sprintf("Failed to create replacement session: %v", err), Type: "error"})
			replacement.ID = ""
			app.session = nil
			return
		}
	}
	if err := app.sessionManager.DeleteSession(id); err != nil {
		tuiApp.Send(tui.StatusMsg{Text: sprintf("Failed to delete session: %v", err), Type: "error"})
		return
	}
	if isActive {
		tuiApp.Send(tui.SessionActivatedMsg{
			SessionID:      app.session.ID,
			Transcript:     nil,
			Model:          app.session.Model,
			Persona:        app.session.Persona,
			Sessions:       app.getSessionInfos(),
			Notice:         sprintf("Deleted session %s; replacement active", shortID(id)),
			NoticeType:     "success",
			SwitchToChat:   true,
			PermissionMode: app.config.PermissionMode.String(),
			EstTokens:      app.session.EstimateTokens(),
		})
	} else {
		tuiApp.Send(tui.SessionsRefreshedMsg{
			Sessions:   app.getSessionInfos(),
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
	path, err := exportSession(session, "", d.app.config.APIKey)
	if err != nil {
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Failed to export: %v", err), Type: "error"})
		return
	}
	absPath, absErr := filepath.Abs(path)
	if absErr != nil {
		absPath = path
	}
	d.tuiApp.Send(tui.SessionsRefreshedMsg{
		Sessions:   d.app.getSessionInfos(),
		Notice:     sprintf("Exported to %s", abbreviatePath(absPath)),
		NoticeType: "success",
	})
}

func (d *tuiSessionsDelegate) OnSessionCopy(id string) {
	session, err := d.app.sessionManager.ReadSession(id)
	if err != nil {
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("Failed to load session for copy: %v", err), Type: "error"})
		return
	}

	content := formatSessionForClipboard(session)
	if err := clipboard.WriteAll(content); err != nil {
		d.tuiApp.Send(tui.SessionsRefreshedMsg{
			Sessions:   d.app.getSessionInfos(),
			Notice:     sprintf("Failed to copy to clipboard: %v (needs xclip or xsel)", err),
			NoticeType: "error",
		})
		return
	}

	d.tuiApp.Send(tui.SessionsRefreshedMsg{
		Sessions:   d.app.getSessionInfos(),
		Notice:     sprintf("Copied %d messages from session %s", len(session.Messages), shortID(id)),
		NoticeType: "success",
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
