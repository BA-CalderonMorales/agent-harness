package main

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
)

// tuiHomeDelegate connects TUI home dashboard to the app.
type tuiHomeDelegate struct {
	app    *App
	tuiApp *tui.App
}

func (d *tuiHomeDelegate) OnDeleteSession(id string) {
	d.app.deleteSession(id, d.tuiApp)
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
	// The export journey: the modal lists the sessions (the same source
	// the Sessions tab uses), the pick runs the existing export, and the
	// bottom notification confirms with the file path. Esc cancels.
	d.tuiApp.OpenExportPicker(d.app.getSessionInfos())
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
