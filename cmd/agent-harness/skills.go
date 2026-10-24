package main

import (
	"path/filepath"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/diag"
	"github.com/BA-CalderonMorales/agent-harness/internal/skills"
)

// Skill discovery.
//
// Skills used to come from two unrelated places: the system prompt
// loaded the project's .agent-harness/skills, while /skills listed a
// different directory entirely. The agent could therefore know about a
// skill the user could not see, and /skills could list a skill the
// agent never received.
//
// One resolver now serves both, and it walks the roots from lowest to
// highest precedence:
//
//   - <config home>/skills          the user's own skills
//   - <workspace>/.workspace/skills the legacy workspace root
//   - <workspace>/.agent-harness/skills  the project's skills
//
// Later roots win, so a project overrides the user-level skill of the
// same name.

// skillDirs returns the skill roots in precedence order.
func (app *App) skillDirs() []string {
	dirs := make([]string, 0, 3)
	dirs = append(dirs, filepath.Join(config.ConfigHome(), "skills"))
	if app.cwd != "" {
		dirs = append(dirs, filepath.Join(app.cwd, ".workspace", "skills"))
		dirs = append(dirs, filepath.Join(app.cwd, ".agent-harness", "skills"))
	}
	return dirs
}

// loadSkills discovers the skills available to this session.
func (app *App) loadSkills() *skills.Registry {
	reg, err := skills.LoadFromDirs(app.skillDirs()...)
	if err != nil {
		diag.Warnf("skills.load", "%v", err)
		return skills.NewRegistry()
	}
	return reg
}

// loadSkillPrompts renders the discovered skills as prompt blocks.
func (app *App) loadSkillPrompts() []string {
	reg := app.loadSkills()
	all := reg.All()
	if len(all) == 0 {
		return nil
	}
	prompts := make([]string, 0, len(all))
	for _, skill := range all {
		prompts = append(prompts, skill.FormatPrompt())
	}
	return prompts
}
