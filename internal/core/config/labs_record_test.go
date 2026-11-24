package config

import "testing"

func TestRecordCompactionIsOffByDefault(t *testing.T) {
	lab, ok := LookupLab(LabRecordCompaction)
	if !ok || lab.EnvVar != "AH_RECORD_COMPACTION" || lab.Default {
		t.Fatalf("LookupLab(record_compaction) = %+v, %v; want the opt-in AH_RECORD_COMPACTION entry", lab, ok)
	}
	dir := isolateLabs(t)
	cfg, err := NewLayeredLoader(dir).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.LabEnabled(LabRecordCompaction) || cfg.LabPinned(LabRecordCompaction) {
		t.Fatal("record compaction is on without a choice or a pin")
	}
}

func TestRecordCompactionEnvironmentPinsItOn(t *testing.T) {
	for _, name := range []string{"AH_RECORD_COMPACTION", "AGENT_HARNESS_RECORD_COMPACTION"} {
		t.Run(name, func(t *testing.T) {
			dir := isolateLabs(t)
			writeProjectSettings(t, dir, `{"record_compaction": false}`)
			t.Setenv(name, "1")
			cfg, err := NewLayeredLoader(dir).Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if !cfg.LabEnabled(LabRecordCompaction) || !cfg.LabPinned(LabRecordCompaction) {
				t.Fatalf("%s=1 did not pin record_compaction on", name)
			}
			if got := cfg.LabPinnedBy(LabRecordCompaction); got != name {
				t.Fatalf("LabPinnedBy = %q, want %q", got, name)
			}
		})
	}
}
