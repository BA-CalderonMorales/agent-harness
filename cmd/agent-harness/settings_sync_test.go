package main

import (
	"reflect"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	tea "github.com/charmbracelet/bubbletea"
)

func settingsRows(app *tui.App) []tui.Setting {
	model := exportedField(reflect.ValueOf(app).Elem().FieldByName("settingsModel"))
	return exportedField(model.FieldByName("settings")).Interface().([]tui.Setting)
}

func flushSettingsMessages(app *tui.App) {
	channel := exportedField(reflect.ValueOf(app).Elem().FieldByName("msgChan"))
	for {
		value, ok := channel.TryRecv()
		if !ok {
			return
		}
		app.Update(value.Interface())
	}
}

func selectSettingsRow(app *tui.App, key string) {
	// Drive the real Settings view through the app's key routing.
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("5")})
	app.Update(tea.KeyMsg{Type: tea.KeyHome})
	for _, setting := range settingsRows(app) {
		if setting.Key == key {
			return
		}
		app.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	panic("setting missing: " + key)
}

func TestPermissionPresetRefreshesVisibleExecuteState(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{PermissionMode: config.PermissionReadOnly, PermRead: true, PermExecute: true, PermExplicit: true}, "model")
	delegate := delegateFor(t, app)
	app.tuiApp.SetSettingsDelegate(delegate)
	app.tuiApp.SetSettings(app.getSettings())
	app.tuiApp.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	selectSettingsRow(app.tuiApp, "permissions")
	app.tuiApp.Update(tea.KeyMsg{Type: tea.KeyRight}) // read-only -> workspace-write
	flushSettingsMessages(app.tuiApp)
	if app.config.PermissionMode != config.PermissionWorkspaceWrite || app.config.PermExecute {
		t.Fatal("preset was not applied")
	}
	for _, row := range settingsRows(app.tuiApp) {
		if row.Key == "perm_execute" && row.BoolValue != app.config.PermExecute {
			t.Fatal("Settings shows Execute enabled while the runtime denies bash")
		}
	}
	selectSettingsRow(app.tuiApp, "perm_execute")
	app.tuiApp.Update(tea.KeyMsg{Type: tea.KeySpace})
	flushSettingsMessages(app.tuiApp)
	if !app.config.PermExecute || app.checkPermissionMode("bash").Behavior == tools.Deny {
		t.Fatal("enabling Execute through the UI did not allow bash")
	}
	restored, err := config.NewLayeredLoader(app.cwd).Load()
	if err != nil || !restored.PermExecute {
		t.Fatal("enabled execution did not survive settings reload")
	}
}

func TestEnabledShellRespectsPerCommandConsent(t *testing.T) {
	for _, consent := range []bool{false, true} {
		name := "rejected"
		if consent {
			name = "approved"
		}
		t.Run(name, func(t *testing.T) {
			app, ui := newProductionPermissionContractApp(t, config.PermissionWorkspaceWrite)
			delegate := &tuiSettingsDelegate{app: app, tuiApp: ui}
			delegate.OnSettingChange("perm_execute", "true")
			flushSettingsMessages(ui)
			app.applyAgentMode(AgentModeManual)
			operation := newPermissionContractOperation(t, app, "bash")
			before := operation.state()
			run := startProductionPermissionContractRun(t, app, ui, operation, true)
			request, completed := waitForPermissionContractApproval(t, ui, run.done)
			if request == nil || completed != nil {
				t.Fatal("enabled shell did not wait for user consent")
			}
			if operation.state() != before {
				t.Fatal("shell executed before consent")
			}
			if consent {
				request.Respond(approval.DecisionApprove)
			} else {
				request.Respond(approval.DecisionReject)
			}
			outcome := waitForPermissionContractOutcome(t, run.done)
			assertPermissionContractResult(t, outcome.events, !consent, "")
			if consent && operation.state() == before {
				t.Fatal("approved bash command did not execute")
			}
			if !consent && operation.state() != before {
				t.Fatal("rejected bash command executed")
			}
		})
	}
}
