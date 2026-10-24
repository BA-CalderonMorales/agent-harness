package main

import (
	"path/filepath"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/agents"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/diag"
)

// Sub-agent definitions.
//
// A definition is a Markdown file with frontmatter under
// .agent-harness/agents (project) or <config home>/agents (user). The
// agent tool's agent_type selects one, so a delegation carries its own
// instructions, tool set, and model instead of being a generic worker
// every time.
//
// The agent tool used to accept an agent_type and then ignore it: the
// type only changed the label on the transcript row.

// agentDirs returns the definition roots in precedence order.
func (app *App) agentDirs() []string {
	dirs := make([]string, 0, 2)
	dirs = append(dirs, filepath.Join(config.ConfigHome(), "agents"))
	if app.cwd != "" {
		dirs = append(dirs, filepath.Join(app.cwd, ".agent-harness", "agents"))
	}
	return dirs
}

// loadAgentDefs discovers the sub-agent definitions available here.
func (app *App) loadAgentDefs() *agents.Registry {
	registry, err := agents.LoadFromDirs(app.agentDirs()...)
	if err != nil {
		diag.Warnf("agents.load", "%v", err)
		return agents.NewRegistry()
	}
	return registry
}

// delegatedSystemPrompt composes the child's instructions: the shared
// harness context, the definition's own instructions when there is one,
// and the bound that keeps a delegated worker from running away.
func delegatedSystemPrompt(base string, definition agents.Definition, hasDefinition bool) string {
	var sb strings.Builder
	sb.WriteString(base)
	if hasDefinition && definition.Prompt != "" {
		sb.WriteString("\n\n")
		sb.WriteString(definition.Prompt)
	}
	sb.WriteString("\n\nYou are a bounded delegated worker. Report exactly what you inspected or changed, and state failures plainly.")
	return sb.String()
}

// subAgentSummaries describes the installed sub-agents for the system
// prompt: name and description, one line each.
func (app *App) subAgentSummaries() []string {
	registry := app.loadAgentDefs()
	if registry.Len() == 0 {
		return nil
	}
	summaries := make([]string, 0, registry.Len())
	for _, def := range registry.All() {
		description := def.Description
		if description == "" {
			description = "(no description)"
		}
		summaries = append(summaries, sprintf("%s: %s", def.Name, description))
	}
	return summaries
}

// formatAgentDefinitions lists the installed sub-agent definitions.
func (app *App) formatAgentDefinitions() string {
	registry := app.loadAgentDefs()
	if registry.Len() == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("Sub-agent definitions:\n")
	for _, def := range registry.All() {
		description := def.Description
		if description == "" {
			description = "(no description)"
		}
		tools := "all tools"
		if len(def.Tools) > 0 {
			tools = strings.Join(def.Tools, ", ")
		}
		sb.WriteString(sprintf("  %-14s %s\n", def.Name, description))
		sb.WriteString(sprintf("  %-14s tools: %s\n", "", tools))
	}
	return sb.String()
}
