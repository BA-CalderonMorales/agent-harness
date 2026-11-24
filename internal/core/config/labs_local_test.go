package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// settings.local.json sits above the user layer. A lab it sets belongs to
// that file: it is never copied into the user layer, and a toggle over it
// is written back to the local file, where it lasts.

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

func readLocalSettings(t *testing.T, dir string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".agent-harness", "settings.local.json"))
	if err != nil {
		t.Fatalf("read local settings: %v", err)
	}
	var values map[string]interface{}
	if err := json.Unmarshal(data, &values); err != nil {
		t.Fatalf("parse local settings: %v", err)
	}
	return values
}

func TestLocalLabIsNotCopiedToUserLayer(t *testing.T) {
	dir := isolateLabs(t)
	writeLocalSettings(t, dir, `{"loop_track": true}`)

	cfg := loadLabs(t, dir)
	if !cfg.LabEnabled(LabLoopTrack) {
		t.Fatal("settings.local.json loop_track=true did not apply")
	}
	if v, ok := cfg.LabSettingsValues()[LabLoopTrack]; ok {
		t.Fatalf("a local-layer lab is in the user-layer save as %v", v)
	}
	if got := cfg.LabLocalValues(); len(got) != 0 {
		t.Fatalf("an untouched local lab would be rewritten: %v", got)
	}
}

func TestLabToggleOverLocalLayerWritesLocalFile(t *testing.T) {
	dir := isolateLabs(t)
	writeLocalSettings(t, dir, `{"loop_track": true, "other": "kept"}`)

	cfg := loadLabs(t, dir)
	cfg.SetLab(LabLoopTrack, false)
	if cfg.LabEnabled(LabLoopTrack) {
		t.Fatal("toggling off over the local layer did not apply in session")
	}
	if v, ok := cfg.LabSettingsValues()[LabLoopTrack]; ok {
		t.Fatalf("a toggle over the local layer went to the user layer as %v", v)
	}
	if err := NewLayeredLoader(dir).SaveSettings(SourceLocal, cfg.LabLocalValues()); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	local := readLocalSettings(t, dir)
	if local[LabLoopTrack] != false || local["other"] != "kept" {
		t.Fatalf("local file = %v, want loop_track false and other keys kept", local)
	}
	if loadLabs(t, dir).LabEnabled(LabLoopTrack) {
		t.Fatal("the toggle over the local layer did not survive a reload")
	}

	cfg = loadLabs(t, dir)
	cfg.ClearLab(LabLoopTrack)
	if err := NewLayeredLoader(dir).SaveSettings(SourceLocal, cfg.LabLocalValues()); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	if _, ok := readLocalSettings(t, dir)[LabLoopTrack]; ok {
		t.Fatal("clearing a local-layer lab left its key in settings.local.json")
	}
}
