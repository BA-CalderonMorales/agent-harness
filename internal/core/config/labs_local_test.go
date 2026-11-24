package config

import (
	"os"
	"path/filepath"
	"testing"
)

// settings.local.json sits above the user layer. A lab it sets belongs to
// that file: it is never copied into the user layer, and the app does not
// change it. Only editing the file changes it.

func writeLocalSettings(t *testing.T, dir, body string) {
	t.Helper()
	path := filepath.Join(dir, ".agent-harness", "settings.local.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write local settings: %v", err)
	}
}

func TestLocalLabIsNotCopiedToUserLayer(t *testing.T) {
	dir := isolateLabs(t)
	writeLocalSettings(t, dir, `{"loop_track": true}`)

	cfg := loadLabs(t, dir)
	if !cfg.LabEnabled(LabLoopTrack) || !cfg.LabSetByLocal(LabLoopTrack) {
		t.Fatal("settings.local.json loop_track=true did not apply as a local-layer value")
	}
	if v, ok := cfg.LabSettingsValues()[LabLoopTrack]; ok {
		t.Fatalf("a local-layer lab is in the user-layer save as %v", v)
	}
}

func TestLabToggleOverLocalLayerIsRefused(t *testing.T) {
	dir := isolateLabs(t)
	writeLocalSettings(t, dir, `{"loop_track": true}`)

	cfg := loadLabs(t, dir)
	cfg.SetLab(LabLoopTrack, false)
	if !cfg.LabEnabled(LabLoopTrack) {
		t.Fatal("SetLab changed a lab settings.local.json sets")
	}
	cfg.ClearLab(LabLoopTrack)
	if !cfg.LabEnabled(LabLoopTrack) {
		t.Fatal("ClearLab changed a lab settings.local.json sets")
	}
	if v, ok := cfg.LabSettingsValues()[LabLoopTrack]; ok {
		t.Fatalf("a refused toggle reached the user-layer save as %v", v)
	}
	if loadLabs(t, dir).LabSetByLocal("no_such_lab") {
		t.Fatal("an unregistered key reports a local-layer value")
	}
}
