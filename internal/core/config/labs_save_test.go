package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Labs choices persist through SaveSettings like every other setting. An
// environment pin stays out of the file; a cleared choice leaves it.

func TestLabSettingsValues(t *testing.T) {
	cfg := &LayeredConfig{}
	if v, ok := cfg.LabSettingsValues()[LabLoopTrack]; !ok || v != nil {
		t.Fatalf("no choice: loop_track = %v (present %v), want nil so the save clears it", v, ok)
	}
	cfg.SetLab(LabLoopTrack, true)
	if v := cfg.LabSettingsValues()[LabLoopTrack]; v != true {
		t.Fatalf("saved choice: loop_track = %v, want true", v)
	}
	cfg.LabsPinned = map[string]bool{LabLoopTrack: false}
	if v, ok := cfg.LabSettingsValues()[LabLoopTrack]; ok {
		t.Fatalf("env-pinned loop_track = %v was included in the save", v)
	}
}

func TestSaveSettingsNilValueRemovesKey(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", configHome)
	path := filepath.Join(configHome, "settings.json")
	if err := os.WriteFile(path, []byte(`{"loop_track": true, "theme": "nord"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := NewLayeredLoader(t.TempDir()).SaveSettings(SourceUser, map[string]interface{}{"loop_track": nil}); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]interface{}
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if v, ok := saved["loop_track"]; ok {
		t.Fatalf("loop_track = %v survived a clearing save", v)
	}
	if saved["theme"] != "nord" {
		t.Fatalf("theme = %v, want the unrelated key kept", saved["theme"])
	}
}
