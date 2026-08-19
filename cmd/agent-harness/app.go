package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/audit"
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
	auditLogger    *audit.Logger
	planMode       bool

	// bootNotice carries a credential/config problem discovered before
	// the TUI exists; run() surfaces it once the TUI is up.
	bootNotice string
}

// newApp creates and initializes a new App instance.
func newApp() (*App, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, errf("failed to get current directory: %w", err)
	}

	app := &App{cwd: cwd}

	// Initialize audit logging (non-fatal if it fails)
	if logger, err := audit.NewLogger(); err == nil {
		app.auditLogger = logger
	}

	if err := app.initConfig(); err != nil {
		return nil, err
	}

	if err := app.initSession(); err != nil {
		return nil, err
	}

	// Git context is collected after the TUI starts (see run) so a slow
	// filesystem never blocks the first paint.
	app.initTools()
	app.initCommands()

	app.client = llm.NewHTTPClientWithBaseURLTimeout(app.config.Provider, app.config.APIKey, app.config.EndpointURL, app.config.HTTPTimeout)
	app.loop = agent.NewLoop(app.client)
	if app.config.ContextLength > 0 {
		app.loop.Config.BlockingTokenLimit = app.config.ContextLength
	}
	app.loop.Config.StreamIdleTimeout = app.config.StreamIdleTimeout

	return app, nil
}

// run starts the TUI mode.
func (app *App) run() error {
	tuiApp := tui.NewApp()
	app.tuiApp = tuiApp

	// Re-register slash commands that need TUI integration
	app.initCommandsForTUI(tuiApp)

	// Set up handlers
	tuiApp.SetUserSubmitHandler(func(text string, ta *tui.App) {
		app.handleUserSubmit(text, ta)
	})
	tuiApp.SetUserCommandHandler(func(cmd string, ta *tui.App) {
		app.handleUserCommand(cmd, ta)
	})
	tuiApp.SetHomeDelegate(&tuiHomeDelegate{app: app, tuiApp: tuiApp})
	tuiApp.SetSessionsDelegate(&tuiSessionsDelegate{app: app, tuiApp: tuiApp})
	tuiApp.SetSettingsDelegate(&tuiSettingsDelegate{app: app, tuiApp: tuiApp})
	tuiApp.SetChatDelegate(&tuiChatDelegate{app: app, tuiApp: tuiApp})
	tuiApp.SetLoginHandler(func(provider, apiKey, model string, ta *tui.App) {
		app.completeLogin(provider, apiKey, model, ta)
	})
	tuiApp.SetLoginModelsProvider(app.wizardModels)
	tuiApp.SetProviderPickHandler(func(provider string, ta *tui.App) {
		app.pickProvider(provider, ta)
	})
	tuiApp.SetGitContextHandler(func(ctx *git.Context, ta *tui.App) {
		app.gitContext = ctx
		ta.SetProjectInfo(app.getProjectInfo())
		ta.ReplaceWelcomeMessage(app.buildWelcomeMessage())
	})

	// Initial data
	tuiApp.SetChatMessages(app.session.Messages)
	welcome := app.buildWelcomeMessage()
	tuiApp.AddMessage("system", welcome)
	tuiApp.RefreshSessions(app.getSessionInfos())
	tuiApp.SetSettings(app.getSettings())
	tuiApp.SetModels(app.getModelItems())
	tuiApp.SetChatModel(app.session.Model)
	tuiApp.SetChatPersona(app.session.Persona)
	tuiApp.SetRuntimeContext(app.config.Provider, app.config.Effort, app.cwd)
	if app.config.Effort == "" {
		app.config.Effort = config.DefaultEffort
	}
	tuiApp.SetProjectInfo(app.getProjectInfo())
	tuiApp.SetHomeStatus(app.session.Model, app.config.PermissionMode.String(), app.session.Persona, app.session.EstimateTokens())
	app.refreshTelemetry(tuiApp)
	tuiApp.SetCommandCompletions(app.cmdRegistry.GetCompletions())
	tuiApp.SetCommands(app.cmdRegistry.GetCommandInfos())

	// Surface boot-time credential/config problems in the TUI (durable
	// system message + misconfigured readiness), then start the probe.
	if app.bootNotice != "" {
		tuiApp.Send(tui.ProviderReadinessMsg{Readiness: 4, Message: app.bootNotice})
	}
	prober := llm.NewHTTPProber(app.config.Provider, app.config.APIKey, app.config.EndpointURL)
	tuiApp.StartProviderProbe(prober)

	// Collect git context off the boot path; the dashboard and welcome
	// fill in when the GitContextMsg lands on the event loop.
	go func() {
		ctx, _ := git.GetContext()
		tuiApp.Send(tui.GitContextMsg{Context: ctx})
	}()

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
