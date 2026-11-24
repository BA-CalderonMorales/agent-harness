package main

import (
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
)

func TestTogglingPinnedRecordCompactionRowIsRefused(t *testing.T) {
	journeyHome(t)
	t.Setenv("AH_RECORD_COMPACTION", "1")
	app := bootJourneyApp(t)
	if !app.config.LabEnabled(config.LabRecordCompaction) {
		t.Fatal("AH_RECORD_COMPACTION=1 did not enable record compaction")
	}
	delegateFor(t, app).OnSettingChange(config.LabRecordCompaction, "false")
	if !app.config.LabEnabled(config.LabRecordCompaction) {
		t.Fatal("the Settings row overrode an env-pinned record compaction lab")
	}
	if _, ok := app.config.LabsSaved[config.LabRecordCompaction]; ok {
		t.Fatal("toggling an env-pinned lab recorded a saved choice")
	}
}
