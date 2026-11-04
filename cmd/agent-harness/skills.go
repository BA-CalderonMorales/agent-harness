package main

import (
	"fmt"
	"path/filepath"
	"strings"

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

// skillContextShare is the fraction of the model's context window skill
// bodies may occupy (1/5 = 20%). The rest belongs to the base prompt,
// tool schemas, conversation history, and the reply. Full bodies are a
// large fixed cost on every request: four project skills alone ran to
// ~25k characters, which is over 6k tokens before the user has typed
// anything. Small local windows and metered hosted calls feel that on
// every turn, so the budget decides between full bodies and a readable
// catalog.
const skillContextShare = 5

// loadSkillPromptsForContext renders the skill blocks the request can
// afford. When the full bodies fit inside skillContextShare of the
// window, they ship verbatim. When they do not, the prompt gets a
// compact catalog with each skill's name, summary, and SKILL.md path:
// the agent already has the read tool, so it can load the one body it
// needs instead of paying for all of them on every turn.
func (app *App) loadSkillPromptsForContext(contextTokens int) []string {
	all := app.loadSkills().All()
	if len(all) == 0 {
		return nil
	}

	full := make([]string, 0, len(all))
	fullChars := 0
	for _, skill := range all {
		block := skill.FormatPrompt()
		full = append(full, block)
		fullChars += len(block)
	}

	// 0 means the window is unknown: keep the full bodies rather than
	// degrading a context we cannot measure.
	if contextTokens <= 0 {
		return full
	}
	if fullChars <= contextTokens*4/skillContextShare {
		return full
	}
	return []string{compactSkillCatalog(all, contextTokens)}
}

// compactSkillCatalog lists the installed skills without their bodies.
func compactSkillCatalog(all []skills.Skill, contextTokens int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n\nSkills are installed, but their full instructions are omitted to fit the %d-token context window. Read a skill's SKILL.md when it applies:\n", contextTokens)
	for _, skill := range all {
		fmt.Fprintf(&b, "- %s — %s\n  %s\n", skill.Name, skillSummary(skill.Content), filepath.Join(skill.Path, "SKILL.md"))
	}
	return b.String()
}

// skillSummary is the one-line description from the skill's frontmatter
// when present, otherwise its first prose line. It keeps the compact
// catalog useful without carrying the body.
func skillSummary(content string) string {
	lines := strings.Split(content, "\n")
	inFrontmatter := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if i == 0 && trimmed == "---" {
			inFrontmatter = true
			continue
		}
		if inFrontmatter {
			if trimmed == "---" {
				break
			}
			if strings.HasPrefix(strings.ToLower(trimmed), "description:") {
				return trimSkillSummary(strings.TrimSpace(trimmed[len("description:"):]))
			}
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ">") || trimmed == "---" {
			continue
		}
		return trimSkillSummary(strings.TrimPrefix(trimmed, "- "))
	}
	return "installed skill"
}

func trimSkillSummary(summary string) string {
	const limit = 220
	if len(summary) <= limit {
		return summary
	}
	return strings.TrimSpace(summary[:limit]) + "…"
}
