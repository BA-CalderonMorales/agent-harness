package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/commands"
	"github.com/BA-CalderonMorales/agent-harness/internal/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/permissions"
	"github.com/BA-CalderonMorales/agent-harness/internal/skills"
	"github.com/BA-CalderonMorales/agent-harness/internal/state"
	"github.com/BA-CalderonMorales/agent-harness/internal/tools"
	"github.com/BA-CalderonMorales/agent-harness/internal/tools/builtin"
	"github.com/BA-CalderonMorales/agent-harness/internal/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/ui"
	"github.com/BA-CalderonMorales/agent-harness/pkg/git"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

var (
	Version   = "0.0.39"
	BuildTime = "unknown"
	GitSHA    = "unknown"
	GitTag    = "unknown"
)

// App holds the application state
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
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "\n%s %v\n", ui.RenderError(err.Error()), "")
		os.Exit(1)
	}
}

func run() error {
	var showVersion, showHelp, useTUI bool
	flag.BoolVar(&showVersion, "version", false, "Show version")
	flag.BoolVar(&showVersion, "v", false, "Show version (shorthand)")
	flag.BoolVar(&showHelp, "help", false, "Show help")
	flag.BoolVar(&showHelp, "h", false, "Show help (shorthand)")
	flag.BoolVar(&useTUI, "tui", true, "Use TUI mode (default: true)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Agent Harness - AI-powered coding assistant\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fmt.Fprintf(os.Stderr, "  -v, --version    Show version\n")
		fmt.Fprintf(os.Stderr, "  -h, --help       Show help\n")
		fmt.Fprintf(os.Stderr, "  --tui=true       Use TUI mode (default)\n")
		fmt.Fprintf(os.Stderr, "  --tui=false      Use CLI mode\n")
		fmt.Fprintf(os.Stderr, "\nFor more: https://github.com/BA-CalderonMorales/agent-harness\n")
	}

	flag.Parse()

	if showVersion {
		printVersion()
		return nil
	}

	if showHelp {
		flag.Usage()
		return nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	app := &App{cwd: cwd}

	if err := app.loadConfig(); err != nil {
		return err
	}

	if err := app.initSession(); err != nil {
		return err
	}

	app.gitContext, _ = git.GetContext()
	app.initTools()

	app.client = llm.NewHTTPClient(app.config.Provider, app.config.APIKey)
	app.loop = agent.NewLoop(app.client)

	app.initSlashCommands()

	if useTUI {
		return app.runTUIMode()
	}
	return app.runCLIMode()
}

func printVersion() {
	buildType := "release"
	if strings.Contains(Version, "dev") || strings.Contains(Version, "local") {
		buildType = "dev"
	}
	fmt.Printf("agent-harness %s\n", Version)
	fmt.Printf("  Build type: %s\n", buildType)
	if GitTag != "unknown" && GitTag != "" {
		fmt.Printf("  Tag: %s\n", GitTag)
	}
	if BuildTime != "unknown" && BuildTime != "" {
		fmt.Printf("  Built: %s\n", BuildTime)
	}
	if GitSHA != "unknown" && GitSHA != "" {
		fmt.Printf("  Git: %s\n", GitSHA)
	}
}

func (app *App) loadConfig() error {
	loader := config.NewLayeredLoader(app.cwd)
	layeredConfig, err := loader.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	app.config = layeredConfig

	credManager := config.NewCredentialManager()
	if credManager.HasSecureCredentials() {
		secureCfg, err := credManager.LoadSecure()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		} else {
			app.secureConfig = secureCfg
			if secureCfg.Provider != "" {
				app.config.Provider = secureCfg.Provider
			}
			if secureCfg.APIKey != "" {
				app.config.APIKey = secureCfg.APIKey
			}
			if secureCfg.Model != "" {
				app.config.Model = secureCfg.Model
			}
		}
	}

	if app.config.APIKey == "" && credManager.HasLegacyCredentials() {
		fmt.Println("Found existing credentials in legacy format.")
		secureCfg, err := credManager.MigrateFromLegacy()
		if err != nil {
			fmt.Printf("Migration failed: %v\n", err)
		} else {
			app.secureConfig = secureCfg
			app.config.Provider = secureCfg.Provider
			app.config.APIKey = secureCfg.APIKey
			app.config.Model = secureCfg.Model
		}
	}

	if app.config.APIKey == "" {
		if err := app.interactiveSetup(credManager); err != nil {
			return fmt.Errorf("setup failed: %w", err)
		}
	}

	return nil
}

func (app *App) initSession() error {
	sessionManager, err := state.NewSessionManager()
	if err != nil {
		return fmt.Errorf("failed to initialize session manager: %w", err)
	}
	app.sessionManager = sessionManager

	model := app.config.Model
	if model == "" {
		model = "nvidia/nemotron-3-super-120b-a12b:free"
	}
	app.session = sessionManager.CreateSession(model)
	app.costTracker = agent.NewCostTracker()
	app.costTracker.SetModel(model)

	return nil
}

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
}

func (app *App) initSlashCommands() {
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
			result := app.session.Compact(state.DefaultCompactionConfig())
			app.session = result.CompactedSession
			return fmt.Sprintf("Compacted: removed %d messages, kept %d", result.RemovedCount, result.KeptCount), nil
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
				path = fmt.Sprintf("session-%s.json", app.session.ID[:8])
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
					return fmt.Sprintf("Error listing sessions: %v", err)
				}
				if len(sessions) == 0 {
					return "No saved sessions."
				}
				var lines []string
				lines = append(lines, "Saved sessions:")
				for _, s := range sessions {
					active := ""
					if s.ID == app.session.ID {
						active = " (active)"
					}
					lines = append(lines, fmt.Sprintf("  %s - %d messages, %d turns%s", s.ID[:8], s.MessageCount, s.Turns, active))
				}
				return strings.Join(lines, "\n")
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

	app.cmdRegistry.Register("version", "Show version",
		commands.VersionHandler(Version, fmt.Sprintf("Built: %s Git: %s", BuildTime, GitSHA)))

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

	app.cmdRegistry.Register("memory", "Show session memory info",
		commands.MemoryHandler(func() string {
			return app.sessionManager.FormatSessionReport()
		}))

	app.cmdRegistry.Register("agents", "Show available agents",
		commands.AgentsHandler(func(args string) string {
			return "Available agents:\n  default  Standard agent with full tool access\n  okabe    Experimental reasoning agent"
		}))

	app.cmdRegistry.Register("skills", "Show available skills",
		commands.SkillsHandler(func(args string) string {
			skillReg, err := skills.LoadFromDirectory(".agent-harness/skills")
			if err != nil {
				return fmt.Sprintf("No skills loaded: %v", err)
			}
			skillsList := skillReg.All()
			if len(skillsList) == 0 {
				return "No skills available in .agent-harness/skills"
			}
			var lines []string
			lines = append(lines, "Available skills:")
			for _, sk := range skillsList {
				desc := sk.Description
				if len(desc) > 60 {
					desc = desc[:57] + "..."
				}
				lines = append(lines, fmt.Sprintf("  %-24s %s", sk.Name, desc))
			}
			return strings.Join(lines, "\n")
		}))

	app.cmdRegistry.Register("reset", "Reset agent harness (delete credentials and all sessions)",
		commands.ResetHandler(func() error {
			credManager := config.NewCredentialManager()
			if err := credManager.ClearSecureConfig(); err != nil {
				return fmt.Errorf("failed to clear credentials: %w", err)
			}
			sessions, err := app.sessionManager.ListSessions()
			if err != nil {
				return fmt.Errorf("failed to list sessions: %w", err)
			}
			for _, s := range sessions {
				path := filepath.Join(app.sessionManager.GetSessionsDir(), s.ID+".json")
				_ = os.Remove(path)
			}
			app.session = app.session.Clear()
			return nil
		}))

	app.cmdRegistry.Register("quit", "Exit the application", commands.QuitHandler())
	app.cmdRegistry.Register("exit", "Exit the application", commands.QuitHandler())
}

func (app *App) initSlashCommandsForTUI(tuiApp *tui.App) {
	// Re-register clear with TUI chat clearing so the viewport actually empties
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

// ---------------------------------------------------------------------------
// TUI Mode
// ---------------------------------------------------------------------------

func (app *App) runTUIMode() error {
	tuiApp := tui.NewApp()

	// Re-register slash commands that need TUI integration
	app.initSlashCommandsForTUI(tuiApp)

	// Set up handlers (non-blocking)
	tuiApp.SetUserSubmitHandler(func(text string, ta tui.App) {
		app.handleUserSubmit(text, &ta)
	})
	tuiApp.SetUserCommandHandler(func(cmd string, ta tui.App) {
		app.handleUserCommand(cmd, &ta)
	})
	tuiApp.SetSessionsDelegate(&TUIsessionsDelegate{app: app, tuiApp: tuiApp})
	tuiApp.SetSettingsDelegate(&TUISettingsDelegate{app: app, tuiApp: tuiApp})
	tuiApp.SetChatDelegate(&TUIChatDelegate{app: app, tuiApp: tuiApp})

	// Initial data
	tuiApp.AddMessage("system", fmt.Sprintf("Agent Harness %s - Type /help for commands", Version))
	tuiApp.RefreshSessions(app.getSessionInfos())
	tuiApp.SetSettings(app.getSettings())
	tuiApp.SetModels(app.getModelItems())

	return tui.Run(tuiApp)
}

// handleUserSubmit handles user message submission (runs in goroutine)
// ALL messages go through the full agent loop - no conversational shortcuts.
// This ensures the LLM always has context and can use tools when appropriate.
func (app *App) handleUserSubmit(text string, tuiApp *tui.App) {
	validator := ui.NewTermuxValidator()
	normalizedInput, valid := validator.ValidateInput(text)
	if !valid {
		tuiApp.Send(tui.AgentErrorMsg{Error: fmt.Errorf("invalid input"), Timestamp: time.Now()})
		return
	}

	userMsg := types.Message{
		UUID:      generateUUID(),
		Role:      types.RoleUser,
		Content:   []types.ContentBlock{types.TextBlock{Text: normalizedInput}},
		Timestamp: time.Now(),
	}
	app.session.AddMessage(userMsg)

	// ALWAYS use the full agent loop - even for greetings.
	// The LLM decides how to respond based on the system prompt.
	app.handleAgentLoopAsync(normalizedInput, tuiApp)
}

// handleUserCommand handles slash commands (runs in goroutine)
func (app *App) handleUserCommand(command string, tuiApp *tui.App) {
	if result, handled, err := app.cmdRegistry.Handle(command); handled {
		if err != nil {
			tuiApp.AddMessage("system", fmt.Sprintf("Error: %v", err))
			return
		}
		if commands.IsQuit(result) {
			tuiApp.Send(tui.QuitMsg{})
			return
		}
		if commands.IsReset(result) {
			tuiApp.AddMessage("system", "Agent Harness has been reset. Encrypted credentials and all session data have been deleted. The application will now exit.")
			tuiApp.Send(tui.QuitMsg{})
			return
		}
		if result != "" {
			tuiApp.AddMessage("system", result)
		}
	} else {
		tuiApp.AddMessage("system", fmt.Sprintf("Unknown command: %s", command))
	}
}

// TUIsessionsDelegate connects TUI sessions to the app
type TUIsessionsDelegate struct {
	app    *App
	tuiApp *tui.App
}

func (d *TUIsessionsDelegate) OnSessionSelect(id string) {
	session, err := d.app.sessionManager.LoadSession(id)
	if err != nil {
		d.tuiApp.AddMessage("system", fmt.Sprintf("Failed to load session: %v", err))
		return
	}
	d.app.session = session
	d.tuiApp.AddMessage("system", fmt.Sprintf("Loaded session %s", id[:8]))
	d.tuiApp.RefreshSessions(d.app.getSessionInfos())
}

func (d *TUIsessionsDelegate) OnSessionDelete(id string) {
	// TODO: Implement
	d.tuiApp.AddMessage("system", "Session deletion not yet implemented")
}

func (d *TUIsessionsDelegate) OnSessionExport(id string) {
	path := fmt.Sprintf("session-%s.json", id[:8])
	if err := d.app.session.SaveToFile(path); err != nil {
		d.tuiApp.AddMessage("system", fmt.Sprintf("Failed to export: %v", err))
		return
	}
	d.tuiApp.AddMessage("system", fmt.Sprintf("Exported to %s", path))
}

func (d *TUIsessionsDelegate) OnSessionLoad() {
	d.tuiApp.RefreshSessions(d.app.getSessionInfos())
}

// TUISettingsDelegate connects TUI settings to the app
type TUISettingsDelegate struct {
	app    *App
	tuiApp *tui.App
}

func (d *TUISettingsDelegate) OnSettingChange(key, value string) {
	switch key {
	case "model":
		d.app.session.Model = value
		d.app.costTracker.SetModel(value)
	case "provider":
		d.app.config.Provider = value
	case "permissions":
		if mode, err := config.ParsePermissionMode(value); err == nil {
			d.app.config.PermissionMode = mode
		}
	}
	d.tuiApp.SetSettings(d.app.getSettings())
}

func (d *TUISettingsDelegate) OnSettingReset() {
	d.tuiApp.AddMessage("system", "Reset to defaults not implemented")
}

func (d *TUISettingsDelegate) OnSettingReload() {
	d.tuiApp.SetSettings(d.app.getSettings())
}

// TUIChatDelegate connects TUI chat to the app
type TUIChatDelegate struct {
	app    *App
	tuiApp *tui.App
}

func (d *TUIChatDelegate) OnSubmit(text string) tea.Cmd {
	return func() tea.Msg {
		d.app.handleUserSubmit(text, d.tuiApp)
		return nil
	}
}

func (d *TUIChatDelegate) OnCommand(command string) {
	d.app.handleUserCommand(command, d.tuiApp)
}

func (app *App) getSessionInfos() []tui.SessionInfo {
	sessions, err := app.sessionManager.ListSessions()
	if err != nil {
		return nil
	}

	var infos []tui.SessionInfo
	for _, s := range sessions {
		infos = append(infos, tui.SessionInfo{
			ID:           s.ID,
			Title:        fmt.Sprintf("Session %s", s.ID[:8]),
			MessageCount: s.MessageCount,
			Turns:        s.Turns,
			CreatedAt:    s.CreatedAt,
			UpdatedAt:    s.UpdatedAt,
			Model:        s.Model,
			IsActive:     s.ID == app.session.ID,
		})
	}
	return infos
}

func (app *App) getSettings() []tui.Setting {
	return []tui.Setting{
		{
			Key:         "model",
			Label:       "Model",
			Value:       app.session.Model,
			Description: "The AI model to use",
			Type:        "string",
		},
		{
			Key:         "provider",
			Label:       "Provider",
			Value:       app.config.Provider,
			Description: "API provider",
			Type:        "string",
		},
		{
			Key:         "permissions",
			Label:       "Permission Mode",
			Value:       app.config.PermissionMode.String(),
			Description: "Tool permission level",
			Type:        "choice",
			Options:     []string{"read-only", "workspace-write", "danger-full-access"},
		},
	}
}

func (app *App) getModelItems() []tui.ModelItem {
	// Return models appropriate for the current provider
	provider := app.config.Provider
	if provider == "" {
		provider = "openrouter"
	}

	var items []tui.ModelItem
	switch provider {
	case "openai":
		items = []tui.ModelItem{
			{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", ContextLen: 128000, IsDefault: app.session.Model == "gpt-4o"},
			{ID: "gpt-4o-mini", Name: "GPT-4o Mini", Provider: "openai", ContextLen: 128000, IsDefault: app.session.Model == "gpt-4o-mini"},
			{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", Provider: "openai", ContextLen: 128000, IsDefault: app.session.Model == "gpt-4-turbo"},
			{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", Provider: "openai", ContextLen: 16385, IsDefault: app.session.Model == "gpt-3.5-turbo"},
		}
	case "anthropic":
		items = []tui.ModelItem{
			{ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet", Provider: "anthropic", ContextLen: 200000, IsDefault: app.session.Model == "claude-3-5-sonnet-20241022"},
			{ID: "claude-3-opus-20240229", Name: "Claude 3 Opus", Provider: "anthropic", ContextLen: 200000, IsDefault: app.session.Model == "claude-3-opus-20240229"},
			{ID: "claude-3-haiku-20240307", Name: "Claude 3 Haiku", Provider: "anthropic", ContextLen: 200000, IsDefault: app.session.Model == "claude-3-haiku-20240307"},
		}
	default:
		// openrouter and fallback
		items = []tui.ModelItem{
			{ID: "nvidia/nemotron-3-super-120b-a12b:free", Name: "Nemotron 3 Super 120B (free)", Provider: "openrouter", ContextLen: 128000, IsDefault: app.session.Model == "nvidia/nemotron-3-super-120b-a12b:free"},
			{ID: "anthropic/claude-3.5-sonnet", Name: "Claude 3.5 Sonnet", Provider: "openrouter", ContextLen: 200000, IsDefault: app.session.Model == "anthropic/claude-3.5-sonnet"},
			{ID: "openai/gpt-4o", Name: "GPT-4o", Provider: "openrouter", ContextLen: 128000, IsDefault: app.session.Model == "openai/gpt-4o"},
			{ID: "openai/gpt-4o-mini", Name: "GPT-4o Mini", Provider: "openrouter", ContextLen: 128000, IsDefault: app.session.Model == "openai/gpt-4o-mini"},
			{ID: "meta-llama/llama-3.1-70b-instruct", Name: "Llama 3.1 70B", Provider: "openrouter", ContextLen: 131072, IsDefault: app.session.Model == "meta-llama/llama-3.1-70b-instruct"},
			{ID: "meta-llama/llama-3.1-8b-instruct", Name: "Llama 3.1 8B", Provider: "openrouter", ContextLen: 131072, IsDefault: app.session.Model == "meta-llama/llama-3.1-8b-instruct"},
			{ID: "google/gemini-pro-1.5", Name: "Gemini Pro 1.5", Provider: "openrouter", ContextLen: 2097152, IsDefault: app.session.Model == "google/gemini-pro-1.5"},
			{ID: "mistralai/mistral-large", Name: "Mistral Large", Provider: "openrouter", ContextLen: 128000, IsDefault: app.session.Model == "mistralai/mistral-large"},
		}
	}

	return items
}

// handleAgentLoopAsync runs the FULL agent loop for ALL user input.
// No more "fast path" - even greetings go through the LLM with tool access.
// This makes the agent truly agentic: it decides how to respond based on context.
func (app *App) handleAgentLoopAsync(input string, tuiApp *tui.App) {
	// Signal start through message channel
	tuiApp.Send(tui.AgentStartMsg{Timestamp: time.Now()})

	sysPrompt := app.buildSystemPrompt()

	toolCtx := tools.Context{
		Options: tools.Options{
			MainLoopModel: app.session.Model,
			Tools:         app.toolRegistry.FilterEnabled(),
			Debug:         false,
		},
		AbortController: context.Background(),
	}

	canUseTool := func(toolName string, toolInput map[string]any, ctx tools.Context) (tools.PermissionDecision, error) {
		t, ok := app.toolRegistry.FindToolByName(toolName)
		if !ok {
			return tools.PermissionDecision{Behavior: tools.Deny, Message: "unknown tool"}, nil
		}

		switch app.config.PermissionMode {
		case config.PermissionReadOnly:
			if !isReadOnlyTool(toolName) {
				return tools.PermissionDecision{
					Behavior: tools.Deny,
					Message:  fmt.Sprintf("Permission denied: %s", toolName),
				}, nil
			}
		case config.PermissionWorkspaceWrite:
			if isDangerousTool(toolName) {
				return tools.PermissionDecision{
					Behavior: tools.Ask,
					Message:  fmt.Sprintf("Confirm: %s", toolName),
				}, nil
			}
		}

		for _, allowed := range app.config.AlwaysAllow {
			if allowed == toolName {
				return tools.PermissionDecision{Behavior: tools.Allow}, nil
			}
		}

		permCtx := permissions.EmptyContext()
		return permissions.Evaluate(t, toolInput, permCtx), nil
	}

	params := agent.QueryParams{
		Messages:       app.session.Messages,
		SystemPrompt:   sysPrompt,
		CanUseTool:     canUseTool,
		ToolUseContext: toolCtx,
	}

	stream, err := app.loop.Query(context.Background(), params)
	if err != nil {
		tuiApp.Send(tui.AgentErrorMsg{Error: err, Timestamp: time.Now()})
		return
	}

	var responseText strings.Builder
	toolCallCount := 0

	for event := range stream {
		switch e := event.(type) {
		case types.StreamMessage:
			for _, block := range e.Message.Content {
				switch b := block.(type) {
				case types.TextBlock:
					// Send each chunk for real-time streaming display
					tuiApp.Send(tui.AgentChunkMsg{
						Text:      b.Text,
						Timestamp: time.Now(),
					})
					responseText.WriteString(b.Text)
				case types.ToolUseBlock:
					toolCallCount++
					tuiApp.Send(tui.AgentToolStartMsg{
						ToolID:   b.ID,
						ToolName: b.Name,
						Input:    b.Input,
					})
				case types.ToolResultBlock:
					tuiApp.Send(tui.AgentToolDoneMsg{
						ToolID:  b.ToolUseID,
						Success: !b.IsError,
						Output:  fmt.Sprintf("%v", b.Content),
					})
				}
			}
			app.session.AddMessage(e.Message)
		}
	}

	app.costTracker.CompleteTurn()

	// Signal completion
	tuiApp.Send(tui.AgentDoneMsg{
		FullResponse: responseText.String(),
		ToolCalls:    toolCallCount,
		Timestamp:    time.Now(),
	})

	// Auto-save check
	if app.session.Turns%5 == 0 {
		if path, err := app.sessionManager.SaveCurrent(); err == nil {
			tuiApp.Send(tui.StatusMsg{Text: fmt.Sprintf("Auto-saved to %s", path), Type: "info"})
		}
	}
}

func (app *App) buildSystemPrompt() string {
	gitContext := ""
	if app.gitContext != nil && app.gitContext.IsRepo {
		gitContext = fmt.Sprintf("Working in: %s", app.gitContext.Root)
		if app.gitContext.Branch != "" {
			gitContext += fmt.Sprintf(" (branch: %s)", app.gitContext.Branch)
		}
	}

	var skillPrompts []string
	skillReg, err := skills.LoadFromDirectory(".agent-harness/skills")
	if err == nil {
		for _, sk := range skillReg.All() {
			skillPrompts = append(skillPrompts, sk.FormatPrompt())
		}
	}

	cfg := agent.SystemPromptConfig{
		PersonaName:      "Agent",
		GitContext:       gitContext,
		PermissionMode:   app.config.PermissionMode.String(),
		WorkingDirectory: app.cwd,
		Skills:           skillPrompts,
	}

	return agent.BuildSystemPrompt(cfg)
}

// ---------------------------------------------------------------------------
// CLI Mode (classic)
// ---------------------------------------------------------------------------

func (app *App) runCLIMode() error {
	gitInfo := &ui.GitInfo{}
	if app.gitContext != nil && app.gitContext.IsRepo {
		gitInfo.IsRepo = true
		gitInfo.Root = app.gitContext.Root
		gitInfo.Branch = app.gitContext.Branch
	}
	if strings.Contains(Version, "dev") || strings.Contains(Version, "local") {
		gitInfo.BuildType = "dev"
	} else {
		gitInfo.BuildType = "release"
	}

	fmt.Print(ui.WelcomeScreen(Version, app.session.Model, app.config.PermissionMode.String(), gitInfo))

	inputHandler := ui.NewContextualInput(app.cmdRegistry.GetCompletions())

	for {
		outcome, err := inputHandler.ReadInput()
		if err != nil {
			return fmt.Errorf("input error: %w", err)
		}

		if outcome.Exit {
			fmt.Print(ui.RenderGoodbye(app.costTracker.Summary()))
			return nil
		}

		if outcome.Cancel {
			fmt.Println("^C")
			continue
		}

		input := outcome.Text
		if input == "" {
			continue
		}

		if result, handled, err := app.cmdRegistry.Handle(input); handled {
			if err != nil {
				fmt.Println(ui.RenderError(err.Error()))
				continue
			}
			if commands.IsQuit(result) {
				fmt.Print(ui.RenderGoodbye(app.costTracker.Summary()))
				return nil
			}
			fmt.Println(result)
			continue
		}

		if err := app.processMessageCLI(input); err != nil {
			fmt.Println(ui.RenderError(fmt.Sprintf("Error: %v", err)))
		}
	}
}

func (app *App) processMessageCLI(input string) error {
	validator := ui.NewTermuxValidator()
	normalizedInput, valid := validator.ValidateInput(input)
	if !valid {
		return fmt.Errorf("invalid input")
	}

	userMsg := types.Message{
		UUID:      generateUUID(),
		Role:      types.RoleUser,
		Content:   []types.ContentBlock{types.TextBlock{Text: normalizedInput}},
		Timestamp: time.Now(),
	}
	app.session.AddMessage(userMsg)

	sysPrompt := app.buildSystemPrompt()
	toolCtx := tools.Context{
		Options: tools.Options{
			MainLoopModel: app.session.Model,
			Tools:         app.toolRegistry.FilterEnabled(),
			Debug:         false,
		},
		AbortController: context.Background(),
	}

	fmt.Println(tui.SpinnerRender("Thinking..."))

	params := agent.QueryParams{
		Messages:       app.session.Messages,
		SystemPrompt:   sysPrompt,
		ToolUseContext: toolCtx,
	}

	stream, err := app.loop.Query(context.Background(), params)
	if err != nil {
		return err
	}

	var hasOutput bool
	for event := range stream {
		switch e := event.(type) {
		case types.StreamMessage:
			if !hasOutput {
				fmt.Print("\r\033[K")
				hasOutput = true
			}
			for _, block := range e.Message.Content {
				if text, ok := block.(types.TextBlock); ok {
					fmt.Print(text.Text)
				}
			}
			app.session.AddMessage(e.Message)
		}
	}

	if hasOutput {
		fmt.Println()
	}

	app.costTracker.CompleteTurn()
	return nil
}

func (app *App) interactiveSetup(credManager *config.CredentialManager) error {
	fmt.Println()
	fmt.Println(ui.HeaderStyle.Render("  Welcome to Agent Harness"))
	fmt.Println()
	fmt.Println("  Let's get you set up.")
	fmt.Println()

	fmt.Println("  Choose an API provider:")
	fmt.Println("    1) OpenRouter")
	fmt.Println("    2) OpenAI")
	fmt.Println("    3) Anthropic")
	fmt.Print("  Enter choice (1-3) [1]: ")

	reader := bufio.NewReader(os.Stdin)
	choice, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read choice: %w", err)
	}
	choice = strings.TrimSpace(choice)

	switch choice {
	case "2":
		app.config.Provider = "openai"
	case "3":
		app.config.Provider = "anthropic"
	default:
		app.config.Provider = "openrouter"
	}

	fmt.Println()
	fmt.Printf("  Selected: %s\n", app.config.Provider)
	fmt.Println()

	fmt.Printf("  Enter your %s API key: ", app.config.Provider)
	apiKey, err := config.PromptPassword("")
	if err != nil {
		return fmt.Errorf("failed to read API key: %w", err)
	}
	apiKey = strings.TrimSpace(apiKey)

	if apiKey == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	app.config.APIKey = apiKey
	fmt.Println("  " + ui.RenderSuccess("API key received"))
	fmt.Println()

	defaultModel := "nvidia/nemotron-3-super-120b-a12b:free"
	if app.config.Provider == "openai" {
		defaultModel = "gpt-4o"
	} else if app.config.Provider == "anthropic" {
		defaultModel = "claude-3-5-sonnet-20241022"
	}

	fmt.Printf("  Model [%s]: ", defaultModel)
	model, err := reader.ReadString('\n')
	if err != nil {
		model = ""
	}
	model = strings.TrimSpace(model)
	if model != "" {
		app.config.Model = model
	} else {
		app.config.Model = defaultModel
	}

	fmt.Println()
	fmt.Println("  Credentials will be encrypted.")
	fmt.Println()

	secureCfg := &config.SecureConfig{
		Provider: app.config.Provider,
		APIKey:   app.config.APIKey,
		Model:    app.config.Model,
	}

	if err := credManager.SaveSecure(secureCfg); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	fmt.Println()
	fmt.Println("  " + ui.RenderSuccess("Credentials saved"))
	fmt.Println()

	return nil
}

func isReadOnlyTool(name string) bool {
	readOnlyTools := []string{"read", "glob", "grep", "search", "web_fetch", "web_search"}
	for _, t := range readOnlyTools {
		if t == name {
			return true
		}
	}
	return false
}

func isDangerousTool(name string) bool {
	dangerousTools := []string{"bash", "write", "edit"}
	for _, t := range dangerousTools {
		if t == name {
			return true
		}
	}
	return false
}

func generateUUID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
