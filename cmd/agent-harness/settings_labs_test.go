package main

import (
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
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
