package config

import (
	"os"
	"path/filepath"
	"testing"
)

// The Labs registry is the only list of experimental flags. Every lookup,
// default, and environment override goes through it.

// isolateLabs points the user layer at a temp dir and clears every Labs
// environment variable so the developer's own shell cannot decide a test.
func isolateLabs(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", filepath.Join(dir, "config-home"))
	for _, lab := range Labs {
		for _, name := range labEnvNames(lab) {
			t.Setenv(name, "")
		}
	}
	return dir
}

func writeProjectSettings(t *testing.T, dir, body string) {
	t.Helper()
	path := filepath.Join(dir, ".agent-harness", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write settings: %v", err)
	}
}

func TestLabsRegistryEntriesAreComplete(t *testing.T) {
	if len(Labs) == 0 {
		t.Fatal("Labs registry is empty; loop_track must be its first entry")
	}
	seen := map[string]bool{}
	for _, lab := range Labs {
		if lab.Key == "" || lab.Label == "" || lab.Description == "" || lab.EnvVar == "" {
			t.Errorf("lab %+v is missing a key, label, description, or env var", lab)
		}
		if seen[lab.Key] {
			t.Errorf("lab key %q is registered twice", lab.Key)
		}
		seen[lab.Key] = true
	}
}

func TestLookupFeedbackLab(t *testing.T) {
	isolateLabs(t)
	lab, ok := LookupLab(LabFeedback)
	if !ok || lab.Key != "feedback" || lab.EnvVar != "AH_FEEDBACK" || lab.Default {
		t.Fatalf("LookupLab(feedback) = %+v, %v; want the opt-in AH_FEEDBACK entry", lab, ok)
	}
	cfg := &LayeredConfig{}
	if cfg.LabEnabled(LabFeedback) {
		t.Fatal("feedback lab on by default, want off")
	}
	t.Setenv("AH_FEEDBACK", "true")
	applyLabEnvOverrides(cfg)
	if !cfg.LabEnabled(LabFeedback) {
		t.Fatal("AH_FEEDBACK=true did not turn the feedback lab on")
	}
}

func TestLookupLab(t *testing.T) {
	lab, ok := LookupLab(LabLoopTrack)
	if !ok || lab.Key != "loop_track" || lab.EnvVar != "AH_LOOP_TRACK" || lab.Default {
		t.Fatalf("LookupLab(loop_track) = %+v, %v; want the opt-in AH_LOOP_TRACK entry", lab, ok)
	}
	if _, ok := LookupLab("no_such_lab"); ok {
		t.Fatal("LookupLab found an unregistered key")
	}
}

func TestLabEnabledPrecedence(t *testing.T) {
	isolateLabs(t)
	cfg := &LayeredConfig{}
	if cfg.LabEnabled(LabLoopTrack) {
		t.Fatal("default: loop_track on, want the registry Default (off)")
	}
	cfg.LabsSaved = map[string]bool{LabLoopTrack: true}
	if !cfg.LabEnabled(LabLoopTrack) {
		t.Fatal("saved true ignored; a saved setting must outrank the Default")
	}
	cfg.LabsPinned = map[string]bool{LabLoopTrack: false}
	if cfg.LabEnabled(LabLoopTrack) {
		t.Fatal("env pin false ignored; the environment must outrank the saved setting")
	}
	if !cfg.LabPinned(LabLoopTrack) {
		t.Fatal("LabPinned = false for an env-pinned lab")
	}
	if cfg.LabEnabled("no_such_lab") {
		t.Fatal("an unregistered lab reports enabled")
	}
}

func TestClearLabRestoresDefault(t *testing.T) {
	cfg := &LayeredConfig{}
	cfg.SetLab(LabLoopTrack, true)
	if !cfg.LabEnabled(LabLoopTrack) {
		t.Fatal("SetLab(true) did not enable the lab")
	}
	cfg.ClearLab(LabLoopTrack)
	if _, ok := cfg.LabsSaved[LabLoopTrack]; ok || cfg.LabEnabled(LabLoopTrack) {
		t.Fatal("ClearLab left a saved choice; clearing must return the lab to its Default")
	}
	// Choosing the Default is the same as clearing: no stored choice.
	cfg.SetLab(LabLoopTrack, true)
	cfg.SetLab(LabLoopTrack, false)
	if _, ok := cfg.LabsSaved[LabLoopTrack]; ok {
		t.Fatal("choosing the Default stored a choice instead of clearing it")
	}
}

func TestLoopTrackLoadsFromSavedSetting(t *testing.T) {
	dir := isolateLabs(t)
	writeProjectSettings(t, dir, `{"loop_track": true}`)
	cfg, err := NewLayeredLoader(dir).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.LabEnabled(LabLoopTrack) || cfg.LabPinned(LabLoopTrack) {
		t.Fatal("an already-saved loop_track setting no longer loads")
	}
}

func TestLoopTrackEnvironmentStillWorks(t *testing.T) {
	for _, name := range []string{"AH_LOOP_TRACK", "AGENT_HARNESS_LOOP_TRACK"} {
		t.Run(name, func(t *testing.T) {
			dir := isolateLabs(t)
			writeProjectSettings(t, dir, `{"loop_track": false}`)
			t.Setenv(name, "1")
			cfg, err := NewLayeredLoader(dir).Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if !cfg.LabEnabled(LabLoopTrack) || !cfg.LabPinned(LabLoopTrack) {
				t.Fatalf("%s=1 did not pin loop_track on", name)
			}
		})
	}
}

func TestLabPinnedByNamesTheVariableThatPinned(t *testing.T) {
	for _, name := range []string{"AH_LOOP_TRACK", "AGENT_HARNESS_LOOP_TRACK"} {
		t.Run(name, func(t *testing.T) {
			dir := isolateLabs(t)
			t.Setenv(name, "1")
			cfg := loadLabs(t, dir)
			if got := cfg.LabPinnedBy(LabLoopTrack); got != name {
				t.Fatalf("LabPinnedBy = %q, want %q (the variable actually set)", got, name)
			}
		})
	}
	if got := (&LayeredConfig{}).LabPinnedBy(LabLoopTrack); got != "" {
		t.Fatalf("unpinned LabPinnedBy = %q, want empty", got)
	}
}

func TestLabEnvironmentIgnoresUnparsableValue(t *testing.T) {
	dir := isolateLabs(t)
	writeProjectSettings(t, dir, `{"loop_track": true}`)
	t.Setenv("AH_LOOP_TRACK", "maybe")
	cfg, err := NewLayeredLoader(dir).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.LabPinned(LabLoopTrack) || !cfg.LabEnabled(LabLoopTrack) {
		t.Fatal("an unparsable env value pinned the lab; it must be ignored like AH_REDUCED_MOTION")
	}
}
