package main

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/core/persona"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/state"
	"github.com/BA-CalderonMorales/agent-harness/internal/skills"
	"strings"
)

// abbreviatePath shortens a path for a one-line status flash: home
// collapses to ~, and over-long paths keep their tail (the filename is
// the part that matters) — a right-truncated path showed the user
// everything except where the file actually was.
func abbreviatePath(path string) string {
	const budget = 48
	if len(path) <= budget {
		return path
	}
	return "…" + path[len(path)-budget+1:]
}

// plural treats 1 as singular; "%d turns" on a one-turn session read
// like a bug report.
func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// formatSessionList formats sessions for display.
func formatSessionList(sessions []state.SessionMetadata, currentID string) string {
	if len(sessions) == 0 {
		return "No saved sessions."
	}
	var lines []string
	lines = append(lines, "Saved sessions:")
	for _, s := range sessions {
		active := ""
		if s.ID == currentID {
			active = " (active)"
		}
		lines = append(lines, sprintf("  %s - %d messages, %d %s%s", s.ID[:8], s.MessageCount, s.Turns, plural(s.Turns, "turn"), active))
	}
	return strings.Join(lines, "\n")
}

// formatSkillsList formats skills for display.
func formatSkillsList(skills []skills.Skill) string {
	if len(skills) == 0 {
		return "No skills available."
	}
	var lines []string
	lines = append(lines, "Available skills:")
	lines = append(lines, "Use /skills <name> to view full content.")
	lines = append(lines, "")
	for _, sk := range skills {
		desc := firstLine(sk.Description)
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		lineCount := strings.Count(sk.Content, "\n") + 1
		lines = append(lines, sprintf("  %-20s %s (%d lines)", sk.Name, desc, lineCount))
	}
	return strings.Join(lines, "\n")
}

// formatSkillDetail shows full content of a single skill.
func formatSkillDetail(sk skills.Skill) string {
	var lines []string
	lines = append(lines, sprintf("Skill: %s", sk.Name))
	lines = append(lines, sprintf("Path:  %s", sk.Path))
	lines = append(lines, "")
	lines = append(lines, sk.Content)
	return strings.Join(lines, "\n")
}

// formatPersonaList formats available personas for display.
func formatPersonaList() string {
	var lines []string
	lines = append(lines, "Available Personas:")
	for _, p := range persona.All() {
		lines = append(lines, sprintf("  %-12s %s - %s", p.String(), p.DisplayName(), p.Description()))
	}
	return strings.Join(lines, "\n")
}

// firstLine extracts the first non-empty line from a string.
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "\n"); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}
