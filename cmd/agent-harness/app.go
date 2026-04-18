package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/state"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/commands"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/services/mcp"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/git"
)

// App holds the application state and coordinates all components.
type App struct {
	config         *config.LayeredConfig
	secureConfig   *config.SecureConfig
	session        *state.Session
	sessionManager *state.SessionManager
	costTracker    *agent.CostTracker
	cmdRegistry    *commands.SlashRegistry
	toolRegistry   *tools.ToolRegistry
	client         llm.Client
	loop           *agent.Loop
	gitContext     *git.Context
	cwd            string
	tuiApp         *tui.App
	executionMode  approval.ExecutionMode
	mcpManager     *mcp.Manager
}

// newApp creates and initializes a new App instance.
func newApp() (*App, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, errf("failed to get current directory: %w", err)
	}

	app := &App{cwd: cwd}

	if err := app.initConfig(); err != nil {
		return nil, err
	}

	if err := app.initSession(); err != nil {
		return nil, err
	}

	app.gitContext, _ = git.GetContext()
	app.initTools()
	app.initCommands()

	app.client = llm.NewHTTPClient(app.config.Provider, app.config.APIKey)
	app.loop = agent.NewLoop(app.client)

	return app, nil
}

// run starts the TUI mode.
func (app *App) run() error {
	tuiApp := tui.NewApp()
	app.tuiApp = tuiApp

	// Re-register slash commands that need TUI integration
	app.initCommandsForTUI(tuiApp)

	// Set up handlers
	tuiApp.SetUserSubmitHandler(func(text string, ta tui.App) {
		app.handleUserSubmit(text, &ta)
	})
	tuiApp.SetUserCommandHandler(func(cmd string, ta tui.App) {
		app.handleUserCommand(cmd, &ta)
	})
	tuiApp.SetSessionsDelegate(&tuiSessionsDelegate{app: app, tuiApp: tuiApp})
	tuiApp.SetSettingsDelegate(&tuiSettingsDelegate{app: app, tuiApp: tuiApp})
	tuiApp.SetChatDelegate(&tuiChatDelegate{app: app, tuiApp: tuiApp})

	// Initial data
	welcome := app.buildWelcomeMessage()
	tuiApp.AddMessage("system", welcome)
	tuiApp.RefreshSessions(app.getSessionInfos())
	tuiApp.SetSettings(app.getSettings())
	tuiApp.SetModels(app.getModelItems())
	tuiApp.SetChatModel(app.session.Model)

	return tui.Run(tuiApp)
}

// getSessionsDir returns the directory where sessions are stored.
func (app *App) getSessionsDir() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "agent-harness", "sessions")
}

// sprintf is a helper for fmt.Sprintf
func sprintf(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}

// errf is a helper for fmt.Errorf
func errf(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}
