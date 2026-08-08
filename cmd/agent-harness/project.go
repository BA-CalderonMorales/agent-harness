package main

import (
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/persona"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// getProjectInfo returns project metadata for the home dashboard.
func (app *App) getProjectInfo() tui.ProjectInfo {
	info := tui.ProjectInfo{
		Name: filepath.Base(app.cwd),
	}

	if app.gitContext != nil && app.gitContext.IsRepo {
		info.GitBranch = app.gitContext.Branch
		info.GitCommit = app.gitContext.Commit
		info.HasChanges = app.gitContext.HasChanges
		info.UncommittedCount = len(app.gitContext.StatusFiles)
		if len(app.gitContext.RecentCommits) > 0 {
			info.LastCommitMsg = app.gitContext.RecentCommits[0]
		}
	}

	info.Type = detectProjectType(app.cwd)
	return info
}

// getWorkspaceInfo returns formatted workspace information.

// buildWelcomeMessage creates a contextual welcome message.
func (app *App) buildWelcomeMessage() string {
	var parts []string
	parts = append(parts, sprintf("Agent Harness %s", Version))

	// Persona-aware greeting
	if app.session != nil && app.session.Persona != "" {
		if p, err := persona.Parse(app.session.Persona); err == nil {
			parts = append(parts, sprintf("  Mode: %s — %s", p.DisplayName(), p.WelcomeGreeting()))
		}
	}

	if app.session != nil && len(app.session.Messages) > 0 {
		parts = append(parts, sprintf("  Resumed session %s (%d messages, %d turns)",
			app.session.ID[:8], len(app.session.Messages), app.session.Turns))
	}

	if app.gitContext != nil && app.gitContext.IsRepo {
		parts = append(parts, sprintf("  Git: %s (%s)", app.gitContext.Root, app.gitContext.Branch))
		if len(app.gitContext.RecentCommits) > 0 {
			parts = append(parts, sprintf("  Last commit: %s", app.gitContext.RecentCommits[0]))
		}
		if app.gitContext.HasChanges {
			parts = append(parts, "  Status: uncommitted changes present")
		} else {
			parts = append(parts, "  Status: clean")
		}
	} else {
		parts = append(parts, sprintf("  Dir: %s", app.cwd))
	}

	if projType := detectProjectType(app.cwd); projType != "" {
		parts = append(parts, sprintf("  Project: %s", projType))
	}

	if app.session.PlanMode {
		parts = append(parts, "  Mode: plan — outline before executing")
	}

	parts = append(parts, "")
	parts = append(parts, "Type /help for commands")
	return strings.Join(parts, "\n")
}

// detectProjectType guesses the project language from common marker files.
func detectProjectType(dir string) string {
	markers := []struct {
		file string
		name string
	}{
		{"go.mod", "Go"},
		{"package.json", "Node"},
		{"Cargo.toml", "Rust"},
		{"pyproject.toml", "Python"},
		{"requirements.txt", "Python"},
		{"composer.json", "PHP"},
		{"Gemfile", "Ruby"},
		{"pom.xml", "Java"},
		{"build.gradle", "Java"},
	}
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(dir, m.file)); err == nil {
			return m.name
		}
	}
	return ""
}

// summarizeMessages sends messages to the LLM for summarization.

func (app *App) getWorkspaceInfo() string {
	var b strings.Builder

	b.WriteString(sprintf("Current directory: %s\n", app.cwd))

	if app.gitContext != nil && app.gitContext.IsRepo {
		b.WriteString(sprintf("Git repository: %s\n", app.gitContext.Root))
		if app.gitContext.Branch != "" {
			b.WriteString(sprintf("  Branch: %s\n", app.gitContext.Branch))
		}
	} else {
		b.WriteString("Git: not a repository\n")
	}

	if app.session != nil {
		b.WriteString(sprintf("\nActive session: %s\n", app.session.ID[:8]))
		b.WriteString(sprintf("  Model: %s\n", app.session.Model))
		b.WriteString(sprintf("  Messages: %d\n", len(app.session.Messages)))
		b.WriteString(sprintf("  Turns: %d\n", app.session.Turns))
	}

	b.WriteString(sprintf("\nPermission mode: %s\n", app.config.PermissionMode.String()))
	b.WriteString(sprintf("Provider: %s\n", app.config.Provider))

	return b.String()
}
func (app *App) initProject(projectType string) (string, error) {
	files := map[string]string{
		"README.md":  fmt.Sprintf("# %s\n\nProject initialized with agent-harness.\n", filepath.Base(app.cwd)),
		".gitignore": "# Agent harness\n.agent-harness/sessions/\nbuild/\ndist/\n\n# OS\n.DS_Store\nThumbs.db\n",
		"LICENSE":    "MIT License\n\nCopyright (c) " + fmt.Sprintf("%d", time.Now().Year()) + "\n\nPermission is hereby granted...\n",
	}

	switch projectType {
	case "go", "Go":
		files["go.mod"] = fmt.Sprintf("module %s\n\ngo 1.24\n", filepath.Base(app.cwd))
		files["main.go"] = "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello, world!\")\n}\n"
		files["Makefile"] = ".PHONY: build run test\n\nbuild:\n\tgo build -o build/app .\n\nrun:\n\tgo run .\n\ntest:\n\tgo test ./...\n"
	case "node", "Node":
		files["package.json"] = fmt.Sprintf("{\n  \"name\": \"%s\",\n  \"version\": \"0.1.0\",\n  \"main\": \"index.js\"\n}\n", filepath.Base(app.cwd))
		files["index.js"] = "console.log('Hello, world!');\n"
	}

	created := []string{}
	for name, content := range files {
		path := filepath.Join(app.cwd, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return "", fmt.Errorf("failed to write %s: %w", name, err)
			}
			created = append(created, name)
		}
	}

	if len(created) == 0 {
		return "No files created — they already exist.", nil
	}
	return fmt.Sprintf("Initialized %s project. Created: %s", projectType, strings.Join(created, ", ")), nil
}
