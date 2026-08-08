package main

import (
	"context"
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/skills"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"strings"
	"time"
)

// buildSystemPrompt constructs the system prompt.
func (app *App) buildSystemPrompt() string {
	gitContext := ""
	if app.gitContext != nil && app.gitContext.IsRepo {
		gitContext = sprintf("%s", app.gitContext.Root)
		if app.gitContext.Branch != "" {
			gitContext += sprintf(" (branch: %s, commit: %s)", app.gitContext.Branch, app.gitContext.Commit)
		}
		if app.gitContext.HasChanges {
			gitContext += " — has uncommitted changes"
		}
	}

	skillPrompts := loadSkillPrompts()

	var recentCommits, statusFiles, topFiles []string
	if app.gitContext != nil && app.gitContext.IsRepo {
		recentCommits = app.gitContext.RecentCommits
		statusFiles = app.gitContext.StatusFiles
		topFiles = app.gitContext.TopLevelFiles
	}

	cfg := agent.SystemPromptConfig{
		PersonaName:      "Agent",
		Persona:          app.session.Persona,
		GitContext:       gitContext,
		PermissionMode:   app.config.PermissionMode.String(),
		WorkingDirectory: app.cwd,
		Skills:           skillPrompts,
		RecentCommits:    recentCommits,
		StatusFiles:      statusFiles,
		TopLevelFiles:    topFiles,
		PlanMode:         app.session.PlanMode,
	}

	return agent.BuildSystemPrompt(cfg)
}

// loadSkillPrompts loads skill prompts from directory.
func loadSkillPrompts() []string {
	skillReg, err := skills.LoadFromDirectory(".agent-harness/skills")
	if err != nil {
		return nil
	}

	var prompts []string
	for _, sk := range skillReg.All() {
		prompts = append(prompts, sk.FormatPrompt())
	}
	return prompts
}

// buildWelcomeMessage creates a contextual welcome message.

// summarizeMessages sends messages to the LLM for summarization.
func (app *App) summarizeMessages(msgs []types.Message) (string, error) {
	if app.client == nil {
		return "", fmt.Errorf("no LLM client")
	}
	var b strings.Builder
	b.WriteString("Summarize the following conversation concisely. Preserve key decisions, facts, and context:\n\n")
	for _, msg := range msgs {
		b.WriteString(fmt.Sprintf("%s: ", msg.Role))
		for _, block := range msg.Content {
			switch blk := block.(type) {
			case types.TextBlock:
				b.WriteString(blk.Text)
			case types.ToolUseBlock:
				b.WriteString(fmt.Sprintf("[tool: %s]", blk.Name))
			case types.ToolResultBlock:
				b.WriteString(fmt.Sprintf("[result: %v]", blk.Content))
			}
		}
		b.WriteString("\n")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := llm.Request{
		Messages: []types.Message{
			{UUID: generateUUID(), Role: types.RoleUser, Content: []types.ContentBlock{types.TextBlock{Text: b.String()}}, Timestamp: time.Now()},
		},
		SystemPrompt: "You are a context summarizer. Summarize conversation history in 2-3 sentences. Be concise but preserve all key facts, decisions, and context.",
		Model:        app.session.Model,
		MaxTokens:    512,
	}

	stream, err := app.client.Stream(ctx, req)
	if err != nil {
		return "", err
	}

	var result strings.Builder
	for event := range stream {
		switch e := event.(type) {
		case types.LLMTextDelta:
			result.WriteString(e.Delta)
		case types.LLMError:
			return result.String(), e.Error
		}
	}
	return strings.TrimSpace(result.String()), nil
}

// initProject scaffolds standard files for a new project.
