package skills

import (
	"os"
	"path/filepath"
	"testing"
)

// writeSkill creates a skill directory with a SKILL.md file.
func writeSkill(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write skill %s: %v", name, err)
	}
}

func TestLoadFromDirsLaterRootWins(t *testing.T) {
	user := t.TempDir()
	project := t.TempDir()
	writeSkill(t, user, "review", "user copy")
	writeSkill(t, user, "only-user", "user only")
	writeSkill(t, project, "review", "project copy")

	reg, err := LoadFromDirs(user, project)
	if err != nil {
		t.Fatalf("LoadFromDirs() error = %v", err)
	}
	review, ok := reg.Get("review")
	if !ok {
		t.Fatal("review skill missing")
	}
	if review.Content != "project copy" {
		t.Fatalf("review content = %q, want the project copy to win", review.Content)
	}
	if _, ok := reg.Get("only-user"); !ok {
		t.Fatal("user-only skill missing")
	}
}

func TestLoadFromDirsSkipsMissingRoots(t *testing.T) {
	present := t.TempDir()
	writeSkill(t, present, "kept", "body")

	reg, err := LoadFromDirs(filepath.Join(t.TempDir(), "missing"), present)
	if err != nil {
		t.Fatalf("LoadFromDirs() error = %v", err)
	}
	if len(reg.All()) != 1 {
		t.Fatalf("skills = %d, want 1", len(reg.All()))
	}
}

func TestRegistryAllIsSorted(t *testing.T) {
	reg := NewRegistry()
	for _, name := range []string{"zebra", "alpha", "middle"} {
		reg.Register(Skill{Name: name})
	}
	got := reg.All()
	want := []string{"alpha", "middle", "zebra"}
	for i, name := range want {
		if got[i].Name != name {
			t.Fatalf("All()[%d] = %q, want %q", i, got[i].Name, name)
		}
	}
}
