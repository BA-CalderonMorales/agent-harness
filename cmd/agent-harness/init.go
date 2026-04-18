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

	credManager := config.NewCredentialManager()
	if err := app.loadCredentials(credManager); err != nil {
		return err
	}

	return nil
}

// loadCredentials handles secure credential loading and migration.
func (app *App) loadCredentials(credManager *config.CredentialManager) error {
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

	// Skip API key check for local providers
	if app.config.APIKey == "" && app.config.Provider != "ollama" && app.config.Provider != "local" {
		if err := app.interactiveSetup(credManager); err != nil {
			return errf("setup failed: %w", err)
		}
	}

	// Set default for local providers
	if app.config.APIKey == "" && (app.config.Provider == "ollama" || app.config.Provider == "local") {
		app.config.APIKey = "ollama"
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
		// Ensure model stays current if config changed
		if app.config.Model != "" && app.config.Model != resumed.Model {
			resumed.Model = app.config.Model
		}
	} else {
		app.session = sessionManager.CreateSession(model)
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
			return nil
		}, nil))

	app.cmdRegistry.Register("compact", "Compact session to reduce token usage",
		commands.CompactHandler(func() (string, error) {
			cfg := state.DefaultCompactionConfig()
			// Wire LLM summarization if client is available
			if app.client != nil {
				cfg.Summarizer = func(msgs []types.Message) (string, error) {
					return app.summarizeMessages(msgs)
				}
			}
			result := app.session.Compact(cfg)
			app.session = result.CompactedSession
			return sprintf("Compacted: removed %d messages, kept %d", result.RemovedCount, result.KeptCount), nil
		}))

	app.cmdRegistry.Register("cost", "Show token usage and cost",
		commands.CostHandler(func() string {
			return app.costTracker.FormatReport()
		}))

	app.cmdRegistry.Register("model", "Show or change the current model",
		commands.ModelHandler(
			func() string { return app.session.Model },
			func(m string) error {
				app.session.Model = m
				app.costTracker.SetModel(m)
				if app.tuiApp != nil {
					app.tuiApp.SetChatModel(m)
				}
				return nil
			},
			func() []string {
				return []string{
					"nvidia/nemotron-3-super-120b-a12b:free",
					"claude-3-5-sonnet-20241022",
					"gpt-4o",
					"gpt-4o-mini",
				}
			},
		))

	app.cmdRegistry.Register("current-model", "Show the current model",
		commands.CurrentModelHandler(func() string { return app.session.Model }))

	app.cmdRegistry.Register("export", "Export conversation to file",
		commands.ExportHandler(func(path string) (string, error) {
			if path == "" {
				path = sprintf("session-%s.json", app.session.ID[:8])
			}
			if err := app.session.SaveToFile(path); err != nil {
				return "", err
			}
			return path, nil
		}))

	app.cmdRegistry.Register("session", "Manage sessions",
		commands.SessionHandler(
			func() string {
				sessions, err := app.sessionManager.ListSessions()
				if err != nil {
					return sprintf("Error listing sessions: %v", err)
				}
				return formatSessionList(sessions, app.session.ID)
			},
			func(id string) error {
				session, err := app.sessionManager.LoadSession(id)
				if err != nil {
					return err
				}
				app.session = session
				return nil
			},
		))

	app.cmdRegistry.Register("diff", "Show git diff",
		commands.DiffHandler(func() string {
			if app.gitContext == nil || !app.gitContext.IsRepo {
				return "Not in a git repository."
			}
			return git.FormatDiff()
		}))

	app.cmdRegistry.Register("commit", "Stage all changes and commit",
		commands.CommitHandler(func(message string) (string, error) {
			if app.gitContext == nil || !app.gitContext.IsRepo {
				return "", fmt.Errorf("not in a git repository")
			}
			repo := git.NewRepo(app.gitContext.Root)
			if err := repo.Add("-A"); err != nil {
				return "", fmt.Errorf("failed to stage changes: %w", err)
			}
			if err := repo.Commit(message); err != nil {
				return "", fmt.Errorf("failed to commit: %w", err)
			}
			branch, _ := repo.CurrentBranch()
			return fmt.Sprintf("Committed to %s: %s", branch, message), nil
		}))

	app.cmdRegistry.Register("plan", "Toggle plan mode",
		commands.PlanHandler(
			func() bool { return app.session.PlanMode },
			func(on bool) string {
				app.session.PlanMode = on
				if on {
					return "Plan mode ON. The agent will outline its approach before executing tools."
				}
				return "Plan mode OFF. The agent will execute tools directly."
			},
		))

	app.cmdRegistry.Register("init", "Initialize project with standard files",
		commands.InitHandler(func(projectType string) (string, error) {
			return app.initProject(projectType)
		}))

	app.cmdRegistry.Register("pr", "Manage pull requests",
		commands.PRHandler(
			func(title, body string) (string, error) {
				if app.gitContext == nil || !app.gitContext.IsRepo {
					return "", fmt.Errorf("not in a git repository")
				}
				if !git.HasGhCLI() {
					return "", fmt.Errorf("gh CLI not found. Install: https://cli.github.com")
				}
				repo := git.NewRepo(app.gitContext.Root)
				return repo.CreatePR(title, body)
			},
			func() (string, error) {
				if app.gitContext == nil || !app.gitContext.IsRepo {
					return "", fmt.Errorf("not in a git repository")
				}
				if !git.HasGhCLI() {
					return "", fmt.Errorf("gh CLI not found. Install: https://cli.github.com")
				}
				repo := git.NewRepo(app.gitContext.Root)
				return repo.ListPRs()
			},
		))

	app.cmdRegistry.Register("branch", "Manage git branches",
		commands.BranchHandler(
			func() (string, error) {
				if app.gitContext == nil || !app.gitContext.IsRepo {
					return "", fmt.Errorf("not in a git repository")
			}
			repo := git.NewRepo(app.gitContext.Root)
			branches, err := repo.ListBranches()
			if err != nil {
				return "", err
			}
			current, _ := repo.CurrentBranch()
			var lines []string
			lines = append(lines, "Branches:")
			for _, b := range branches {
				b = strings.TrimSpace(b)
				marker := "  "
				if strings.TrimPrefix(b, "* ") == current {
					marker = "● "
					b = strings.TrimPrefix(b, "* ")
				} else {
					b = strings.TrimPrefix(b, "  ")
				}
				lines = append(lines, marker+b)
			}
			return strings.Join(lines, "\n"), nil
		},
		func(name string) (string, error) {
			if app.gitContext == nil || !app.gitContext.IsRepo {
				return "", fmt.Errorf("not in a git repository")
			}
			repo := git.NewRepo(app.gitContext.Root)
			if err := repo.CreateBranch(name); err != nil {
				return "", err
			}
			return fmt.Sprintf("Created and switched to branch: %s", name), nil
		},
		func(name string) (string, error) {
			if app.gitContext == nil || !app.gitContext.IsRepo {
				return "", fmt.Errorf("not in a git repository")
			}
			repo := git.NewRepo(app.gitContext.Root)
			if err := repo.SwitchBranch(name); err != nil {
				return "", err
			}
			return fmt.Sprintf("Switched to branch: %s", name), nil
		},
		func(name string) (string, error) {
			if app.gitContext == nil || !app.gitContext.IsRepo {
				return "", fmt.Errorf("not in a git repository")
			}
			repo := git.NewRepo(app.gitContext.Root)
			if err := repo.DeleteBranch(name); err != nil {
				return "", err
			}
			return fmt.Sprintf("Deleted branch: %s", name), nil
		},
		))

	app.cmdRegistry.Register("version", "Show version",
		commands.VersionHandler(Version, sprintf("Built: %s Git: %s", BuildTime, GitSHA)))

	app.cmdRegistry.Register("config", "Show configuration",
		commands.ConfigHandler(func() string {
			if app.config == nil {
				return "No configuration loaded."
			}
			return app.config.GetConfigReport()
		}))

	app.cmdRegistry.Register("permissions", "Show or change permission mode",
		commands.PermissionsHandler(
			func() string { return app.config.PermissionMode.String() },
			func(m string) error {
				mode, err := config.ParsePermissionMode(m)
				if err != nil {
					return err
				}
				app.config.PermissionMode = mode
				return nil
			},
			func() string {
				if app.config == nil {
					return "No configuration loaded."
				}
				return app.config.GetPermissionReport()
			},
		))

	app.cmdRegistry.Register("agents", "Show available agents",
		commands.AgentsHandler(func(args string) string {
			return "Available agents:\n  default  Standard agent with full tool access\n  okabe    Experimental reasoning agent"
		}))

	app.cmdRegistry.Register("skills", "Show available skills",
		commands.SkillsHandler(func(args string) string {
			skillReg, err := skills.LoadFromDirectory(".agent-harness/skills")
			if err != nil {
				return sprintf("No skills loaded: %v", err)
			}
			skillsList := skillReg.All()
			if len(skillsList) == 0 {
				return "No skills available in .agent-harness/skills"
			}
			return formatSkillsList(skillsList)
		}))

	app.cmdRegistry.Register("workspace", "Show workspace information",
		commands.WorkspaceHandler(func() string {
			return app.getWorkspaceInfo()
		}))

	app.cmdRegistry.Register("reset", "Reset agent harness",
		commands.ResetHandler(func() error {
			return app.reset()
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
				return nil
			},
			func() {
				tuiApp.Send(tui.ClearChatMsg{})
			},
		))
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
	var lines []string
	lines = append(lines, "Available skills:")
	for _, sk := range skills {
		desc := sk.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		lines = append(lines, sprintf("  %-24s %s", sk.Name, desc))
	}
	return strings.Join(lines, "\n")
}
