package main

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
)

// labsCategory is the Settings section that lists the Labs registry. It
// is always the last section.
const labsCategory = "Labs"

// labsMarker opens every unlocked Labs row's description so even a narrow
// pane's clipped detail line says the flag is not a promise. A locked row
// leads with its lock reason instead.
const labsMarker = "Experimental, may be removed. "

// labsLocalLock locks a row whose lab settings.local.json sets: the app
// does not edit that file, so the toggle is refused.
const labsLocalLock = "Set by settings.local.json. Edit that file to change it."

// labsSettings generates one bool row per config.Labs entry, in registry
// order. Nothing here names a particular lab.
func (app *App) labsSettings() []tui.Setting {
	rows := make([]tui.Setting, 0, len(config.Labs))
	for _, lab := range config.Labs {
		// The detail panel keeps only the first line, so a reason the
		// row is locked comes first: otherwise it is clipped away.
		desc := labsMarker + lab.Description
		switch {
		case app.config.LabPinned(lab.Key):
			desc = "Pinned by " + app.config.LabPinnedBy(lab.Key) + " for this session. " + desc
		case app.config.LabSetByLocal(lab.Key):
			desc = labsLocalLock + " " + desc
		}
		rows = append(rows, tui.Setting{
			Key:         lab.Key,
			Label:       lab.Label,
			Category:    labsCategory,
			Description: desc,
			Type:        "bool",
			BoolValue:   app.config.LabEnabled(lab.Key),
		})
	}
	return rows
}

// handleLabChange applies a Labs row toggle. An env-pinned lab refuses
// the change: the environment owns it for this session. A lab
// settings.local.json sets refuses it too, and nothing is written. The
// view already flipped the row; OnSettingChange resends the settings after
// every change, refused or not, which puts the real value back.
func (d *tuiSettingsDelegate) handleLabChange(lab config.Lab, value string) {
	if d.app.config.LabPinned(lab.Key) {
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("%s is pinned by %s for this session.", lab.Label, d.app.config.LabPinnedBy(lab.Key)), Type: "warning"})
		return
	}
	if d.app.config.LabSetByLocal(lab.Key) {
		d.tuiApp.Send(tui.StatusMsg{Text: sprintf("%s: %s", lab.Label, labsLocalLock), Type: "warning"})
		return
	}
	d.app.config.SetLab(lab.Key, value == "true")
	d.app.commitConfigChange()
	d.app.applyLabs(d.tuiApp)
	d.tuiApp.Send(tui.StatusMsg{Text: sprintf("%s (Labs): %s", lab.Label, boolToEnabled(d.app.config.LabEnabled(lab.Key))), Type: "info"})
}

// applyLabs pushes every lab's effective value to the interface. A lab
// that changes the UI wires its setter here, and graduating it deletes
// the line.
func (app *App) applyLabs(tuiApp *tui.App) {
	if tuiApp == nil {
		return
	}
	tuiApp.SetLoopTrack(app.config.LabEnabled(config.LabLoopTrack))
}
