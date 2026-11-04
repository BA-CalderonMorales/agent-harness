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

// TestSkillPromptsCompactOverBudget pins the hosted/local performance
// fix: full skill bodies are a fixed multi-thousand-token tax on every
// request, so a small window gets a catalog it can afford instead. The
// path is real, so the agent can read the body on demand.
func TestSkillPromptsCompactOverBudget(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_CONFIG_HOME", "")

	workspace := filepath.Join(home, "workspace")
	skillDir := filepath.Join(workspace, ".agent-harness", "skills", "debugging-patterns")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := "---\nname: debugging-patterns\ndescription: Systematic root cause analysis for tricky bugs.\n---\n\n# Debugging\n\n" + strings.Repeat("trace the failure to its source. ", 400)
	skillPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	app := &App{cwd: workspace}
	full := app.loadSkillPromptsForContext(1_000_000)
	if len(full) != 1 || !strings.Contains(full[0], "trace the failure") {
		t.Fatalf("large window must keep the full body: %v", full)
	}

	compact := app.loadSkillPromptsForContext(4096)
	if len(compact) != 1 {
		t.Fatalf("compact catalog = %v, want one block", compact)
	}
	got := compact[0]
	if strings.Contains(got, "trace the failure") {
		t.Fatalf("small window must not carry the full body:\n%s", got)
	}
	for _, want := range []string{"debugging-patterns", "Systematic root cause analysis", skillPath} {
		if !strings.Contains(got, want) {
			t.Fatalf("compact catalog missing %q:\n%s", want, got)
		}
	}
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("catalog path is not readable: %v", err)
	}
}
