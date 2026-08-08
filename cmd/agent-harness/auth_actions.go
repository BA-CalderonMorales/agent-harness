package main

import (
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"os"
	"path/filepath"
)

// requireGitRepo returns an error if the app is not inside a git repository.
func (app *App) requireGitRepo() error {
	if app.gitContext == nil || !app.gitContext.IsRepo {
		return fmt.Errorf("not in a git repository")
	}
	return nil
}

// logout clears credentials from memory and secure storage.
func (app *App) logout() error {
	app.config.APIKey = ""
	app.secureConfig = nil
	credManager := config.NewCredentialManager()
	if err := credManager.ClearSecureConfig(); err != nil {
		return errf("failed to clear credentials: %w", err)
	}
	return nil
}

// startLogin begins the TUI login wizard.
func (app *App) startLogin() error {
	if app.tuiApp == nil {
		return errf("login wizard only available in TUI mode")
	}
	app.loginState = loginProvider
	app.tuiApp.AddMessage("system", "Login wizard started.\nChoose provider:\n  1) Local OpenAI-compatible (llama.cpp)\n  2) OpenAI\n  3) Anthropic\n  4) OpenRouter\n  5) Ollama\nEnter choice (1-5) [1]:")
	return nil
}

// reset clears all credentials and sessions.
func (app *App) reset() error {
	credManager := config.NewCredentialManager()
	if err := credManager.ClearSecureConfig(); err != nil {
		return errf("failed to clear credentials: %w", err)
	}
	sessions, err := app.sessionManager.ListSessions()
	if err != nil {
		return errf("failed to list sessions: %w", err)
	}
	for _, s := range sessions {
		path := filepath.Join(app.getSessionsDir(), s.ID+".json")
		_ = os.Remove(path)
	}
	app.session = app.session.Clear()
	return nil
}

// formatSessionList formats sessions for display.
