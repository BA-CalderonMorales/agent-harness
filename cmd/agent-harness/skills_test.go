package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The system prompt and the /skills command must describe the same set
// of skills. They used to read different directories, so the agent
// could know a skill the user could not list, and /skills could list a
// skill the agent never received.

func TestSkillDirsPrecedence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_CONFIG_HOME", "")

	app := &App{cwd: filepath.Join(home, "workspace")}
	dirs := app.skillDirs()
	if len(dirs) != 3 {
		t.Fatalf("skillDirs() = %v, want three roots", dirs)
	}
	if !strings.HasSuffix(dirs[0], filepath.Join("config", "skills")) {
		t.Fatalf("skillDirs()[0] = %q, want the user config home first", dirs[0])
	}
	if !strings.HasSuffix(dirs[2], filepath.Join(".agent-harness", "skills")) {
		t.Fatalf("skillDirs()[2] = %q, want the project skills last", dirs[2])
	}
}

func TestLoadSkillsFindsProjectSkills(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_CONFIG_HOME", "")

	workspace := filepath.Join(home, "workspace")
	skillDir := filepath.Join(workspace, ".agent-harness", "skills", "release-workflow")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("ship the ritual"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	app := &App{cwd: workspace}
	reg := app.loadSkills()
	skill, ok := reg.Get("release-workflow")
	if !ok {
		t.Fatalf("skills = %v, want release-workflow", reg.All())
	}
	if skill.Content != "ship the ritual" {
		t.Fatalf("content = %q", skill.Content)
	}

	prompts := app.loadSkillPrompts()
	if len(prompts) != 1 || !strings.Contains(prompts[0], "ship the ritual") {
		t.Fatalf("prompts = %v, want the skill in the system prompt", prompts)
	}
}
