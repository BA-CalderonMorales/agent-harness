package main

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/persona"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/state"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
)

// initConfig loads configuration from all sources.
func (app *App) initConfig() error {
	loader := config.NewLayeredLoader(app.cwd)
	layeredConfig, err := loader.Load()
	if err != nil {
		return errf("failed to load configuration: %w", err)
	}
	app.config = layeredConfig
	if app.config.Effort == "" {
		app.config.Effort = config.DefaultEffort
	}
	workspacePath, err := resolveWorkspacePath(app.cwd, app.config.WorkspacePath)
	if err != nil {
		return errf("invalid workspace_path: %w", err)
	}
	app.cwd = workspacePath
	app.syncGranularPermissions()

	// Resolve secret references (secret://env|file|cmd:...) before the
	// credential flows run, so env vars, the config file, and the secure
	// store all see the concrete key. A broken reference must not leak a
	// literal into a request: fail the resolution loudly and boot
	// misconfigured with a pointer to /login.
	resolvedKey, err := config.ResolveSecret(app.config.APIKey)
	if err != nil {
		app.bootNotice = sprintf("Failed to resolve api_key secret reference: %v. Fix agent-harness.yml or run /login.", err)
		app.config.APIKey = ""
	} else {
		app.config.APIKey = resolvedKey
	}

	credManager := config.NewCredentialManager()
	if err := app.loadCredentials(credManager); err != nil {
		return err
	}

	return nil
}
func (app *App) initSession() error {
	// An explicit SessionDir (config or env) pins the whole root;
	// otherwise sessions scope themselves to this project.
	var sessionManager *state.SessionManager
	var err error
	if app.config.SessionDir != "" {
		sessionManager, err = state.NewSessionManagerWithDir(app.config.SessionDir)
	} else {
		sessionManager, err = state.NewSessionManagerForProject(app.cwd)
	}
	if err != nil {
		return errf("failed to initialize session manager: %w", err)
	}
	app.sessionManager = sessionManager

	model := app.config.Model
	if model == "" {
		model = "nvidia/nemotron-3-super-120b-a12b:free"
	}

	// Try to resume the most recent session for continuity
	if resumed, ok := sessionManager.ResumeLatestSession(); ok {
		app.session = resumed
		// The session keeps the model last used in it; adopt it as the
		// running configuration instead of overwriting it, so a model
		// picked in a previous session is not forgotten on restart.
		// An env-pinned model (AH_MODEL) outranks the session, though:
		// automation and demo boots must get the model they asked for.
		if app.config.ModelPinned {
			resumed.Model = app.config.Model
		} else {
			app.syncModelFields()
		}
		// Apply configured persona if valid and session has no persona set
		if app.config.Persona != "" && resumed.Persona == "" {
			if p, err := persona.Parse(app.config.Persona); err == nil {
				resumed.Persona = p.String()
			}
		}
	} else {
		app.session = sessionManager.CreateSession(model)
		// Apply configured persona to new session
		if app.config.Persona != "" {
			if p, err := persona.Parse(app.config.Persona); err == nil {
				app.session.Persona = p.String()
			}
		}
	}

	app.costTracker = agent.NewCostTracker()
	app.costTracker.SetModel(app.session.Model)
	app.initExecutionMode()

	return nil
}

// initExecutionMode sets up the execution mode from config.
func (app *App) initExecutionMode() {
	if app.config.ExecutionMode != "" {
		if mode, err := approval.ParseExecutionMode(app.config.ExecutionMode); err == nil {
			app.executionMode = mode
		} else {
			app.executionMode = approval.ModeInteractive
		}
	} else if app.config.PermissionMode == config.PermissionDangerFullAccess {
		app.executionMode = approval.ModeYolo
	} else {
		app.executionMode = approval.ModeInteractive
	}
}
func (app *App) initCommands() {
	app.initCommandsCore()
	app.initCommandsGit()
	app.initCommandsSystem()
}
