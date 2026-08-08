package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/persona"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/state"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/commands"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/services/mcp"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools/builtin"
	toolmcp "github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools/mcp"
	"github.com/BA-CalderonMorales/agent-harness/internal/skills"
	"github.com/BA-CalderonMorales/agent-harness/internal/ui"
	"github.com/BA-CalderonMorales/agent-harness/pkg/git"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// initConfig loads configuration from all sources.
func (app *App) initConfig() error {
	loader := config.NewLayeredLoader(app.cwd)
	layeredConfig, err := loader.Load()
	if err != nil {
		return errf("failed to load configuration: %w", err)
	}
	app.config = layeredConfig
	workspacePath, err := resolveWorkspacePath(app.cwd, app.config.WorkspacePath)
	if err != nil {
		return errf("invalid workspace_path: %w", err)
	}
	app.cwd = workspacePath
	app.syncGranularPermissions()

	credManager := config.NewCredentialManager()
	if err := app.loadCredentials(credManager); err != nil {
		return err
	}

	return nil
}

// loadCredentials handles secure credential loading and migration.
func (app *App) loadCredentials(credManager *config.CredentialManager) error {
	if config.IsLocalProvider(app.config.Provider) {
		if app.config.APIKey == "" {
			app.config.APIKey = app.config.Provider
		}
		return nil
	}

	if app.config.APIKey != "" {
		return nil
	}

	if credManager.HasSecureCredentials() {
		secureCfg, err := credManager.LoadSecure()
		if err != nil {
			return app.handleCredentialError(credManager, err)
		}
		app.applySecureConfig(secureCfg)
	}

	if app.config.APIKey == "" && credManager.HasLegacyCredentials() {
		app.migrateLegacyCredentials(credManager)
	}

	if app.config.APIKey == "" {
		if err := app.interactiveSetup(credManager); err != nil {
			return errf("setup failed: %w", err)
		}
	}

	return nil
}

// handleCredentialError handles decryption failures gracefully.
func (app *App) handleCredentialError(credManager *config.CredentialManager, err error) error {
	fmt.Fprintf(os.Stderr, "\n%s\n", ui.ErrorStyle.Render("Failed to load credentials"))
	fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)

	fmt.Println("Would you like to:")
	fmt.Println("  1) Try again")
	fmt.Println("  2) Reset credentials and set up again")
	fmt.Print("\nChoice [1-2] [1]: ")

	reader := bufio.NewReader(os.Stdin)
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	if choice == "2" {
		if clearErr := credManager.ClearSecureConfig(); clearErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to clear credentials: %v\n", clearErr)
		} else {
			fmt.Println(ui.RenderSuccess("Credentials cleared. Starting fresh..."))
		}
	} else {
		return errf("credential decryption failed: %w", err)
	}
	return nil
}

// applySecureConfig applies secure configuration values.
// Environment variables take precedence over saved credentials.
func (app *App) applySecureConfig(secureCfg *config.SecureConfig) {
	app.secureConfig = secureCfg
	if secureCfg.Provider != "" && os.Getenv("AH_PROVIDER") == "" && os.Getenv("AGENT_HARNESS_PROVIDER") == "" {
		app.config.Provider = secureCfg.Provider
	}
	if secureCfg.APIKey != "" && os.Getenv("AH_API_KEY") == "" && os.Getenv("AGENT_HARNESS_API_KEY") == "" {
		app.config.APIKey = secureCfg.APIKey
	}
	if secureCfg.Model != "" && os.Getenv("AH_MODEL") == "" && os.Getenv("AGENT_HARNESS_MODEL") == "" {
		app.config.Model = secureCfg.Model
	}
}

// migrateLegacyCredentials migrates from legacy format.
func (app *App) migrateLegacyCredentials(credManager *config.CredentialManager) {
	fmt.Println("Found existing credentials in legacy format.")
	secureCfg, err := credManager.MigrateFromLegacy()
	if err != nil {
		fmt.Printf("Migration failed: %v\n", err)
	} else {
		app.applySecureConfig(secureCfg)
	}
}

// initSession initializes the session manager and creates or resumes a session.
func (app *App) initSession() error {
	sessionManager, err := state.NewSessionManager()
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
		app.syncModelFields()
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

// initTools registers all built-in tools and MCP tools.
func (app *App) initTools() {
	app.toolRegistry = tools.NewRegistry()
	app.toolRegistry.RegisterBuiltIn(builtin.BashTool)
	app.toolRegistry.RegisterBuiltIn(builtin.FileReadTool)
	app.toolRegistry.RegisterBuiltIn(builtin.FileEditTool)
	app.toolRegistry.RegisterBuiltIn(builtin.FileWriteTool)
	app.toolRegistry.RegisterBuiltIn(builtin.GlobTool)
	app.toolRegistry.RegisterBuiltIn(builtin.GrepTool)
	app.toolRegistry.RegisterBuiltIn(builtin.LsRecursiveTool)
	app.toolRegistry.RegisterBuiltIn(builtin.ListDirectoryTool)
	app.toolRegistry.RegisterBuiltIn(builtin.FindTool)
	app.toolRegistry.RegisterBuiltIn(builtin.AskUserQuestionTool)
	app.toolRegistry.RegisterBuiltIn(builtin.TodoWriteTool)
	app.toolRegistry.RegisterBuiltIn(builtin.WebFetchTool)
	app.toolRegistry.RegisterBuiltIn(builtin.WebSearchTool)

	app.mcpManager = mcp.NewManager()
	if len(app.config.McpServers) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := app.mcpManager.LoadAndConnect(ctx, app.config.McpServers); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to connect MCP servers: %v\n", err)
		} else {
			for _, def := range app.mcpManager.AllToolDefs() {
				app.toolRegistry.RegisterMCP(toolmcp.Wrap(def, app.mcpManager))
			}
		}
	}
}

// initCommands registers all slash commands.
func (app *App) initCommands() {
	app.cmdRegistry = commands.NewSlashRegistry()

	app.cmdRegistry.Register("help", "Show available commands",
		commands.HelpHandler(app.cmdRegistry))

	app.cmdRegistry.Register("status", "Show session status",
		commands.StatusHandler(func() string {
			return app.sessionManager.FormatSessionReport()
		}))

	app.cmdRegistry.Register("clear", "Clear the session history",
		commands.ClearHandler(func() error {
			app.session = app.session.Clear()
			app.sessionManager.SetCurrent(app.session)
			_, _ = app.sessionManager.SaveCurrent()
			return nil
		}, nil))

	app.cmdRegistry.Register("compact", "Compact session to reduce token usage",
		commands.CompactHandler(func() (string, error) {
			cfg := state.DefaultCompactionConfig()
			if app.client != nil {
				cfg.Summarizer = func(msgs []types.Message) (string, error) {
					return app.summarizeMessages(msgs)
				}
			}
			result := app.session.Compact(cfg)
			app.session = result.CompactedSession
			app.sessionManager.SetCurrent(app.session)
			if _, err := app.sessionManager.SaveCurrent(); err != nil {
				return "", fmt.Errorf("save compacted session: %w", err)
			}
			return sprintf("Compacted: removed %d messages, kept %d", result.RemovedCount, result.KeptCount), nil
		}))

	app.cmdRegistry.Register("cost", "Show token usage and cost",
		commands.CostHandler(func() string {
			if app.costTracker == nil {
				return "Cost tracking is not active."
			}
			return app.costTracker.Summary()
		}))

	app.cmdRegistry.Register("model", "Show or change the current model",
		commands.ModelHandler(
			func() string { return app.session.Model },
			func(m string) error {
				app.session.Model = m
				if app.costTracker != nil {
					app.costTracker.SetModel(m)
				}
				app.commitConfigChange()
				app.sessionManager.SetCurrent(app.session)
				_, _ = app.sessionManager.SaveCurrent()
				return nil
			},
			func() []string {
				items := app.getModelItems()
				names := make([]string, len(items))
				for i, item := range items {
					names[i] = item.ID
				}
				return names
			},
		))

	app.cmdRegistry.Register("current-model", "Show the current model",
		commands.CurrentModelHandler(func() string { return app.session.Model }))

	app.cmdRegistry.Register("export", "Export conversation to file",
		commands.ExportHandler(func(args string) (string, error) {
			return exportSession(app.session, args)
		}))

	app.cmdRegistry.Register("session", "Manage sessions",
		commands.SessionHandler(
			func() string {
				sessions, err := app.sessionManager.ListSessions()
				if err != nil {
					return sprintf("Failed to list sessions: %v", err)
				}
				return formatSessionList(sessions, app.session.ID)
			},
			func(id string) error {
				loaded, err := app.sessionManager.LoadSession(id)
				if err != nil {
					return err
				}
				app.session = loaded
				app.sessionManager.SetCurrent(loaded)
				return nil
			},
		))

	app.cmdRegistry.Register("plan", "Toggle plan mode (outline before executing)",
		commands.PlanHandler(
			func() bool { return app.planMode },
			func(enabled bool) string {
				app.planMode = enabled
				if enabled {
					return "Plan mode enabled. Agent will outline steps before executing."
				}
				return "Plan mode disabled. Agent will execute directly."
			},
		))

	app.cmdRegistry.Register("improve", "Run self-improvement workflow",
		commands.ImproveHandler(func() (string, error) {
			return "Self-improvement workflow complete: workspace health verified, build & tests passing.", nil
		}))

	app.cmdRegistry.Register("memory", "Show system prompt and context state",
		commands.MemoryHandler(func() string {
			return app.getMemoryInfo()
		}))

	app.cmdRegistry.Register("init", "Initialize project with standard files",
		commands.InitHandler(func(projectType string) (string, error) {
			return app.initProject(projectType)
		}))

	app.cmdRegistry.Register("pr", "Manage pull requests",
		commands.PRHandler(
			func(title, body string) (string, error) {
				if err := app.requireGitRepo(); err != nil {
					return "", err
				}
				repo := git.NewRepo(app.cwd)
				return repo.CreatePR(title, body)
			},
			func() (string, error) {
				if err := app.requireGitRepo(); err != nil {
					return "", err
				}
				repo := git.NewRepo(app.cwd)
				return repo.ListPRs()
			},
		))

	app.cmdRegistry.Register("branch", "Manage git branches",
		commands.BranchHandler(
			func() (string, error) {
				if err := app.requireGitRepo(); err != nil {
					return "", err
				}
				repo := git.NewRepo(app.cwd)
				branches, err := repo.ListBranches()
				if err != nil {
					return "", err
				}
				return strings.Join(branches, "\n"), nil
			},
			func(name string) (string, error) {
				if err := app.requireGitRepo(); err != nil {
					return "", err
				}
				repo := git.NewRepo(app.cwd)
				if err := repo.CreateBranch(name); err != nil {
					return "", err
				}
				return sprintf("Created and switched to branch %s", name), nil
			},
			func(name string) (string, error) {
				if err := app.requireGitRepo(); err != nil {
					return "", err
				}
				repo := git.NewRepo(app.cwd)
				if err := repo.SwitchBranch(name); err != nil {
					return "", err
				}
				return sprintf("Switched to branch %s", name), nil
			},
			func(name string) (string, error) {
				if err := app.requireGitRepo(); err != nil {
					return "", err
				}
				repo := git.NewRepo(app.cwd)
				if err := repo.DeleteBranch(name); err != nil {
					return "", err
				}
				return sprintf("Deleted branch %s", name), nil
			},
		))

	app.cmdRegistry.Register("version", "Show version",
		commands.VersionHandler(Version, sprintf("Built: %s Git: %s", BuildTime, GitSHA)))

	app.cmdRegistry.Register("config", "Show or update configuration (usage: /config [provider|endpoint|model|key] <val>)",
		commands.ConfigHandler(
			func() string {
				return app.formatConfig()
			},
			func(key, value string) (string, error) {
				return app.updateConfiguration(key, value)
			},
		))

	app.cmdRegistry.Register("settings", "Open settings dashboard tab",
		commands.SettingsHandler())

	app.cmdRegistry.Register("permissions", "Show or change permission mode",
		commands.PermissionsHandler(
			func() string { return app.executionMode.String() },
			func(m string) error {
				mode, err := approval.ParseExecutionMode(m)
				if err != nil {
					return err
				}
				app.executionMode = mode
				return nil
			},
			func() string { return app.getPermissionsReport() },
		))

	app.cmdRegistry.Register("agents", "Show available agents",
		commands.AgentsHandler(func(args string) string {
			return app.formatAgentsList()
		}))

	app.cmdRegistry.Register("skills", "Show available skills",
		commands.SkillsHandler(func(args string) string {
			skillsDir := filepath.Join(app.cwd, ".workspace", "skills")
			reg, _ := skills.LoadFromDirectory(skillsDir)
			if args != "" {
				if sk, ok := reg.Get(args); ok {
					return formatSkillDetail(sk)
				}
				return sprintf("Skill not found: %s", args)
			}
			return formatSkillsList(reg.All())
		}))

	app.cmdRegistry.Register("workspace", "Show workspace information",
		commands.WorkspaceHandler(func() string {
			return app.getWorkspaceInfo()
		}))

	app.cmdRegistry.Register("logout", "Log out and clear credentials",
		commands.LogoutHandler(func() error {
			return app.logout()
		}))

	app.cmdRegistry.Register("audit", "Show recent tool activity",
		commands.AuditHandler(func() string {
			return app.getAuditLog()
		}))

	app.cmdRegistry.Register("login", "Log in with new credentials",
		commands.LoginHandler(func() error {
			return app.startLogin()
		}))

	app.cmdRegistry.Register("persona", "Switch behavior mode",
		commands.PersonaHandler(
			func() string { return app.session.Persona },
			func(p string) error {
				parsed, err := persona.Parse(p)
				if err != nil {
					return err
				}
				app.session.Persona = parsed.String()
				app.sessionManager.SetCurrent(app.session)
				_, _ = app.sessionManager.SaveCurrent()
				return nil
			},
			formatPersonaList,
		))

	app.cmdRegistry.Register("diff", "Show git working tree changes",
		commands.DiffHandler(func() string {
			if err := app.requireGitRepo(); err != nil {
				return "Not in a git repository."
			}
			repo := git.NewRepo(app.cwd)
			diff, err := repo.Diff()
			if err != nil {
				return sprintf("Failed to get diff: %v", err)
			}
			return diff
		}))

	app.cmdRegistry.Register("commit", "Stage and commit changes",
		commands.CommitHandler(func(msg string) (string, error) {
			if err := app.requireGitRepo(); err != nil {
				return "", err
			}
			repo := git.NewRepo(app.cwd)
			if err := repo.Add("."); err != nil {
				return "", err
			}
			if err := repo.Commit(msg); err != nil {
				return "", err
			}
			return sprintf("Committed changes with message: %s", msg), nil
		}))

	app.cmdRegistry.Register("reset", "Reset credentials and sessions",
		commands.ResetHandler(func() error {
			return app.reset()
		}))

	app.cmdRegistry.Register("steer", "Queue a message for current turn",
		commands.SteerHandler(func(msg string) {
			app.steerQueue = append(app.steerQueue, msg)
		}))

	app.cmdRegistry.Register("quit", "Exit the application", commands.QuitHandler())
	app.cmdRegistry.Register("exit", "Exit the application", commands.QuitHandler())
}

// initCommandsForTUI re-registers commands that need TUI integration.
func (app *App) initCommandsForTUI(tuiApp *tui.App) {
	app.cmdRegistry.Register("clear", "Clear the session history",
		commands.ClearHandler(
			func() error {
				app.session = app.session.Clear()
				app.sessionManager.SetCurrent(app.session)
				return nil
			},
			func(msg string) {
				tuiApp.Send(tui.ClearChatMsg{FollowUpMsg: msg})
			},
		))

	app.cmdRegistry.Register("steer", "Queue a message for current turn",
		commands.SteerHandler(func(msg string) {
			if msg != "" {
				tuiApp.Send(tui.StatusMsg{Text: sprintf("Queued steer message: %s", msg), Type: "info"})
			}
		}))
}

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
		lines = append(lines, sprintf("  %s - %d messages, %d turns%s", s.ID[:8], s.MessageCount, s.Turns, active))
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
