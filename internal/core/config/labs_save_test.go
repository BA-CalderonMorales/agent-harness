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

// loadLabs reloads every layer the way a fresh boot does.
func loadLabs(t *testing.T, dir string) *LayeredConfig {
	t.Helper()
	cfg, err := NewLayeredLoader(dir).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return cfg
}

// A lab a project layer turns on must be possible to turn off for good:
// the user layer stores the explicit choice, and clearing it falls back
// to the inherited project value.
func TestLabToggleOverProjectLayerIsDurable(t *testing.T) {
	dir := isolateLabs(t)
	writeProjectSettings(t, dir, `{"loop_track": true}`)

	cfg := loadLabs(t, dir)
	if !cfg.LabEnabled(LabLoopTrack) {
		t.Fatal("project layer loop_track=true did not apply")
	}
	if v := cfg.LabSettingsValues()[LabLoopTrack]; v != nil {
		t.Fatalf("an inherited project value would be copied into the user layer as %v", v)
	}

	cfg.SetLab(LabLoopTrack, false)
	if err := NewLayeredLoader(dir).SaveSettings(SourceUser, cfg.LabSettingsValues()); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	if loadLabs(t, dir).LabEnabled(LabLoopTrack) {
		t.Fatal("toggled off over a project layer, but the next boot turned it back on")
	}

	cfg = loadLabs(t, dir)
	cfg.ClearLab(LabLoopTrack)
	if !cfg.LabEnabled(LabLoopTrack) {
		t.Fatal("clearing did not fall back to the inherited project value")
	}
	if err := NewLayeredLoader(dir).SaveSettings(SourceUser, cfg.LabSettingsValues()); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	if !loadLabs(t, dir).LabEnabled(LabLoopTrack) {
		t.Fatal("after clearing, the next boot did not inherit the project value")
	}
}

// Without a lower layer, choosing the Default leaves no key behind.
func TestLabToggleToDefaultWithoutLowerLayerRemovesKey(t *testing.T) {
	dir := isolateLabs(t)
	cfg := loadLabs(t, dir)
	cfg.SetLab(LabLoopTrack, true)
	cfg.SetLab(LabLoopTrack, false)
	if v, ok := cfg.LabSettingsValues()[LabLoopTrack]; !ok || v != nil {
		t.Fatalf("loop_track = %v (present %v), want nil so the save removes the key", v, ok)
	}
}
