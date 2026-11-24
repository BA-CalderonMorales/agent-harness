package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	tea "github.com/charmbracelet/bubbletea"
)

// Labs choices reach settings.json through the one settings write path,
// commitConfigChange, under the same pin rule as the provider.

func TestCommitConfigChangePersistsAndClearsLabs(t *testing.T) {
	journeyHome(t)
	app := bootJourneyApp(t)

	app.config.SetLab(config.LabLoopTrack, true)
	app.commitConfigChange()
	if v := readUserSettings(t)[config.LabLoopTrack]; v != true {
		t.Fatalf("loop_track = %v after enabling, want true saved", v)
	}

	app.config.ClearLab(config.LabLoopTrack)
	app.commitConfigChange()
	if v, ok := readUserSettings(t)[config.LabLoopTrack]; ok {
		t.Fatalf("loop_track = %v after clearing, want the key removed", v)
	}
	if reloaded := bootJourneyApp(t); reloaded.config.LabEnabled(config.LabLoopTrack) {
		t.Fatal("a cleared lab came back on after a restart")
	}
}

func TestCommitConfigChangeSkipsEnvPinnedLabs(t *testing.T) {
	journeyHome(t)
	t.Setenv("AH_LOOP_TRACK", "1")
	app := bootJourneyApp(t)
	if !app.config.LabEnabled(config.LabLoopTrack) {
		t.Fatal("AH_LOOP_TRACK=1 did not enable the loop track")
	}

	// A choice recorded while pinned (by code, not the refused Settings
	// row) must still stay out of the file: the write path owns the rule.
	app.config.SetLab(config.LabLoopTrack, true)
	app.config.Theme = "nord"
	app.commitConfigChange()
	saved := readUserSettings(t)
	if saved["theme"] != "nord" {
		t.Fatalf("theme = %v, want the unrelated change saved", saved["theme"])
	}
	if v, ok := saved[config.LabLoopTrack]; ok {
		t.Fatalf("env-pinned loop_track = %v was written to settings.json", v)
	}
}

// The Settings tab lists one bool row per registry entry, last, under the
// Labs header. The rows come from config.Labs, not from a hand list.

func TestLabsSettingsRowsComeFromRegistry(t *testing.T) {
	journeyHome(t)
	app := bootJourneyApp(t)
	app.config.SetLab(config.LabLoopTrack, true)
	rows := app.getSettings()

	var labs []tui.Setting
	for i, row := range rows {
		if row.Category == labsCategory {
			labs = append(labs, row)
		} else if len(labs) > 0 {
			t.Fatalf("row %d (%s) follows the Labs section; Labs must be listed last", i, row.Key)
		}
	}
	if len(labs) != len(config.Labs) {
		t.Fatalf("Labs rows = %d, want one per registry entry (%d)", len(labs), len(config.Labs))
	}
	for i, lab := range config.Labs {
		row := labs[i]
		if row.Key != lab.Key || row.Label != lab.Label || row.Type != "bool" {
			t.Errorf("row %d = %s/%s/%s, want bool row %s/%s", i, row.Key, row.Label, row.Type, lab.Key, lab.Label)
		}
		if row.BoolValue != app.config.LabEnabled(lab.Key) {
			t.Errorf("row %s shows %v, want the effective value %v", lab.Key, row.BoolValue, app.config.LabEnabled(lab.Key))
		}
		if !strings.Contains(row.Description, lab.Description) || !strings.Contains(strings.ToLower(row.Description), "experimental, may be removed") {
			t.Errorf("row %s description %q lacks the lab description or the experimental marker", lab.Key, row.Description)
		}
	}
}

func TestTogglingLabRowPersists(t *testing.T) {
	journeyHome(t)
	app := bootJourneyApp(t)
	delegate := delegateFor(t, app)

	delegate.OnSettingChange(config.LabLoopTrack, "true")
	if !app.config.LabEnabled(config.LabLoopTrack) || readUserSettings(t)[config.LabLoopTrack] != true {
		t.Fatal("toggling the Labs row on did not enable and save loop_track")
	}
	delegate.OnSettingChange(config.LabLoopTrack, "false")
	if v, ok := readUserSettings(t)[config.LabLoopTrack]; ok || app.config.LabEnabled(config.LabLoopTrack) {
		t.Fatalf("toggling back to the default left loop_track = %v saved", v)
	}
}

// A project layer that turns a lab on must not win back a Settings
// toggle on the next boot.
func TestTogglingLabOffOverProjectLayerSurvivesRestart(t *testing.T) {
	journeyHome(t)
	if err := os.MkdirAll(".agent-harness", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(".agent-harness", "settings.json"), []byte(`{"loop_track": true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	app := bootJourneyApp(t)
	if !app.config.LabEnabled(config.LabLoopTrack) {
		t.Fatal("project layer loop_track=true did not apply")
	}

	delegateFor(t, app).OnSettingChange(config.LabLoopTrack, "false")
	if v := readUserSettings(t)[config.LabLoopTrack]; v != false {
		t.Fatalf("user layer loop_track = %v, want an explicit false over the project value", v)
	}
	if bootJourneyApp(t).config.LabEnabled(config.LabLoopTrack) {
		t.Fatal("the project layer turned loop_track back on after a restart")
	}
}

// settings.local.json owns a lab it sets: an unrelated save leaves the
// user layer alone, and a Settings toggle is written to the local file.
func TestLocalLayerLabStaysInLocalFile(t *testing.T) {
	journeyHome(t)
	local := filepath.Join(".agent-harness", "settings.local.json")
	if err := os.MkdirAll(".agent-harness", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(local, []byte(`{"loop_track": true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	app := bootJourneyApp(t)

	app.config.Theme = "nord"
	app.commitConfigChange()
	if v, ok := readUserSettings(t)[config.LabLoopTrack]; ok {
		t.Fatalf("an unrelated save copied the local loop_track = %v into the user layer", v)
	}

	delegateFor(t, app).OnSettingChange(config.LabLoopTrack, "false")
	if v, ok := readUserSettings(t)[config.LabLoopTrack]; ok {
		t.Fatalf("the toggle wrote loop_track = %v to the user layer, not the local file", v)
	}
	data, err := os.ReadFile(local)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"loop_track": false`) {
		t.Fatalf("settings.local.json = %s, want loop_track false", data)
	}
	if bootJourneyApp(t).config.LabEnabled(config.LabLoopTrack) {
		t.Fatal("the toggle over settings.local.json did not survive a restart")
	}
}

func TestTogglingPinnedLabRowIsRefused(t *testing.T) {
	journeyHome(t)
	t.Setenv("AH_LOOP_TRACK", "0")
	app := bootJourneyApp(t)
	delegate := delegateFor(t, app)

	delegate.OnSettingChange(config.LabLoopTrack, "true")
	if app.config.LabEnabled(config.LabLoopTrack) {
		t.Fatal("the Settings row overrode an env-pinned lab")
	}
	if _, ok := app.config.LabsSaved[config.LabLoopTrack]; ok {
		t.Fatal("toggling an env-pinned lab recorded a saved choice")
	}
	data, err := os.ReadFile(filepath.Join(config.ConfigHome(), "settings.json"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if strings.Contains(string(data), config.LabLoopTrack) {
		t.Fatalf("toggling an env-pinned lab wrote it to settings.json:\n%s", data)
	}
}

// statusTexts drains the TUI's message channel and returns every status
// line the app sent.
func statusTexts(app *tui.App) []string {
	channel := exportedField(reflect.ValueOf(app).Elem().FieldByName("msgChan"))
	var texts []string
	for {
		value, ok := channel.TryRecv()
		if !ok {
			return texts
		}
		if status, ok := value.Interface().(tui.StatusMsg); ok {
			texts = append(texts, status.Text)
		}
	}
}

// The pin names the variable that set it, not always the AH_* spelling.
func TestPinnedLabNamesTheVariableThatPinnedIt(t *testing.T) {
	journeyHome(t)
	t.Setenv("AGENT_HARNESS_LOOP_TRACK", "1")
	app := bootJourneyApp(t)

	for _, row := range app.getSettings() {
		if row.Key != config.LabLoopTrack {
			continue
		}
		if !strings.Contains(row.Description, "AGENT_HARNESS_LOOP_TRACK") || strings.Contains(row.Description, "AH_LOOP_TRACK") {
			t.Fatalf("row description %q does not name AGENT_HARNESS_LOOP_TRACK alone", row.Description)
		}
	}

	delegate := delegateFor(t, app)
	delegate.OnSettingChange(config.LabLoopTrack, "false")
	texts := strings.Join(statusTexts(app.tuiApp), "\n")
	if !strings.Contains(texts, "pinned by AGENT_HARNESS_LOOP_TRACK") || strings.Contains(texts, "AH_LOOP_TRACK") {
		t.Fatalf("pin warning does not name AGENT_HARNESS_LOOP_TRACK alone:\n%s", texts)
	}
}

func TestLabsSectionRendersInTerminal(t *testing.T) {
	for _, size := range [][2]int{{40, 15}, {80, 24}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			journeyHome(t)
			app := bootJourneyApp(t)
			screen := newJourneyScreen(t, wireJourneyTUI(t, app), size[0], size[1])
			screen.Keys("5")
			screen.SendAndPump(tea.KeyMsg{Type: tea.KeyHome})
			screen.Press("up") // wraps to the last row: the Labs section
			screen.AssertFitsWidth(size[0])
			screen.AssertContains(config.Labs[len(config.Labs)-1].Label, "Experimental")
			// At 40x15 the list window is one row tall, so the focused row
			// scrolls its section header out of view (true of every
			// section); the header is checked where there is room for it.
			if size[1] >= 24 {
				screen.AssertContains(labsCategory)
			}
		})
	}
}
