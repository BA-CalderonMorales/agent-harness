package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/commands"
	"github.com/BA-CalderonMorales/agent-harness/internal/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/permissions"
	"github.com/BA-CalderonMorales/agent-harness/internal/skills"
	"github.com/BA-CalderonMorales/agent-harness/internal/state"
	"github.com/BA-CalderonMorales/agent-harness/internal/tools"
	"github.com/BA-CalderonMorales/agent-harness/internal/tools/builtin"
	"github.com/BA-CalderonMorales/agent-harness/internal/ui"
	"github.com/BA-CalderonMorales/agent-harness/pkg/git"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

var (
	Version   = "0.0.9"
	BuildTime = "unknown"
	GitSHA    = "unknown"
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
		fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Initialize app
	app := &App{cwd: cwd}

	// Load layered configuration
	loader := config.NewLayeredLoader(cwd)
	layeredConfig, err := loader.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	app.config = layeredConfig

	// Try to load secure credentials
	credManager := config.NewCredentialManager()
	if credManager.HasSecureCredentials() {
		secureCfg, err := credManager.LoadSecure()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load secure credentials: %v\n", err)
		} else {
			app.secureConfig = secureCfg
			// Override config with secure credentials
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

	// Check for legacy credentials and migrate
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

	// Interactive setup if still no API key
	if app.config.APIKey == "" {
		if err := app.interactiveSetup(credManager); err != nil {
			return fmt.Errorf("setup failed: %w", err)
		}
	}

	// Initialize session manager
	sessionManager, err := state.NewSessionManager()
	if err != nil {
		return fmt.Errorf("failed to initialize session manager: %w", err)
	}
	app.sessionManager = sessionManager

	// Create or load session
	model := app.config.Model
	if model == "" {
		model = "anthropic/claude-3.5-sonnet"
	}
	app.session = sessionManager.CreateSession(model)
	app.costTracker = agent.NewCostTracker()
	app.costTracker.SetModel(model)

	// Initialize git context
	app.gitContext, _ = git.GetContext()

	// Initialize tool registry
	app.toolRegistry = tools.NewRegistry()
	app.registerTools()

	// Initialize LLM client
	app.client = llm.NewHTTPClient(app.config.Provider, app.config.APIKey)
	app.loop = agent.NewLoop(app.client)

	// Initialize slash commands
	app.initSlashCommands()

	// Print welcome
	app.printWelcome()

	// Main REPL loop
	return app.runREPL()
}

func (app *App) registerTools() {
	app.toolRegistry.RegisterBuiltIn(builtin.BashTool)
	app.toolRegistry.RegisterBuiltIn(builtin.FileReadTool)
	app.toolRegistry.RegisterBuiltIn(builtin.FileEditTool)
	app.toolRegistry.RegisterBuiltIn(builtin.FileWriteTool)
	app.toolRegistry.RegisterBuiltIn(builtin.NotebookEditTool)
	app.toolRegistry.RegisterBuiltIn(builtin.RewindTool)
	app.toolRegistry.RegisterBuiltIn(builtin.GlobTool)
	app.toolRegistry.RegisterBuiltIn(builtin.GrepTool)
	app.toolRegistry.RegisterBuiltIn(builtin.AskUserQuestionTool)
	app.toolRegistry.RegisterBuiltIn(builtin.TodoWriteTool)
	app.toolRegistry.RegisterBuiltIn(builtin.AgentTool)
	app.toolRegistry.RegisterBuiltIn(builtin.WebFetchTool)
	app.toolRegistry.RegisterBuiltIn(builtin.WebSearchTool)
	app.toolRegistry.RegisterBuiltIn(builtin.EnterPlanModeTool)
	app.toolRegistry.RegisterBuiltIn(builtin.ExitPlanModeTool)
	app.toolRegistry.RegisterBuiltIn(builtin.ExportTool)
	app.toolRegistry.RegisterBuiltIn(builtin.SearchTranscriptTool)
	app.toolRegistry.RegisterBuiltIn(builtin.SettingsTool)
}

func (app *App) initSlashCommands() {
	app.cmdRegistry = commands.NewSlashRegistry()

	// Help
	app.cmdRegistry.Register("help", "Show available commands",
		commands.HelpHandler(app.cmdRegistry))

	// Status
	app.cmdRegistry.Register("status", "Show session and workspace status",
		commands.StatusHandler(app.getStatusReport))

	// Clear
	app.cmdRegistry.Register("clear", "Clear the session history",
		commands.ClearHandler(func() error {
			app.session = app.session.Clear()
			return nil
		}))

	// Compact
	app.cmdRegistry.Register("compact", "Compact session to reduce token usage",
		commands.CompactHandler(func() (string, error) {
			result := app.session.Compact(state.DefaultCompactionConfig())
			app.session = result.CompactedSession
			return ui.RenderCompactReport(result.RemovedCount, result.KeptCount, result.Skipped), nil
		}))

	// Cost
	app.cmdRegistry.Register("cost", "Show token usage and cost",
		commands.CostHandler(func() string {
			return app.costTracker.FormatReport()
		}))

	// Model
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
					"claude-3-5-sonnet-20241022",
					"claude-3-5-haiku-20241022",
					"claude-3-opus-20240229",
					"gpt-4o",
					"gpt-4o-mini",
				}
			},
		))

	// Permissions
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
				modes := []struct {
					Name        string
					Description string
					Current     bool
				}{
					{"read-only", "Read/search tools only", app.config.PermissionMode == config.PermissionReadOnly},
					{"workspace-write", "Edit files inside the workspace", app.config.PermissionMode == config.PermissionWorkspaceWrite},
					{"danger-full-access", "Unrestricted tool access", app.config.PermissionMode == config.PermissionDangerFullAccess},
				}
				return ui.RenderPermissionsReport(app.config.PermissionMode.String(), modes)
			},
		))

	// Config
	app.cmdRegistry.Register("config", "Show configuration",
		commands.ConfigHandler(func() string {
			return app.config.GetConfigReport()
		}))

	// Diff
	app.cmdRegistry.Register("diff", "Show git diff of workspace changes",
		commands.DiffHandler(func() string {
			return git.FormatDiff()
		}))

	// Export
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

	// Session
	app.cmdRegistry.Register("session", "Manage sessions",
		commands.SessionHandler(
			func() string {
				sessions, err := app.sessionManager.ListSessions()
				if err != nil {
					return fmt.Sprintf("Error listing sessions: %v", err)
				}
				if len(sessions) == 0 {
					return "No saved sessions found."
				}
				var lines []string
				lines = append(lines, "Saved sessions:")
				for _, s := range sessions {
					lines = append(lines, fmt.Sprintf("  %s - %d messages, %d turns (%s)",
						s.ID[:8], s.MessageCount, s.Turns, s.UpdatedAt.Format("2006-01-02")))
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

	// Version
	app.cmdRegistry.Register("version", "Show version information",
		commands.VersionHandler(Version, fmt.Sprintf("Build: %s (%s)", BuildTime, GitSHA)))

	// Quit
	app.cmdRegistry.Register("quit", "Exit the application", commands.QuitHandler())
	app.cmdRegistry.Register("exit", "Exit the application", commands.QuitHandler())
}

func (app *App) printWelcome() {
	fmt.Println()
	fmt.Println(ui.HeaderStyle.Render("╔════════════════════════════════════════════════════════════╗"))
	fmt.Println(ui.HeaderStyle.Render("║     Agent Harness                                          ║"))
	fmt.Printf( "%s %s\n", ui.HeaderStyle.Render("║"), ui.DimStyle.Render(fmt.Sprintf("Version %s", Version)))
	fmt.Println(ui.HeaderStyle.Render("╚════════════════════════════════════════════════════════════╝"))
	fmt.Println()
	fmt.Printf("Model: %s\n", app.session.Model)
	fmt.Printf("Permissions: %s\n", app.config.PermissionMode.String())
	
	if app.gitContext != nil && app.gitContext.IsRepo {
		fmt.Printf("Workspace: %s\n", app.gitContext.FormatStatus())
	}
	
	fmt.Println()
	fmt.Println("Type /help for available commands, or enter your message.")
	fmt.Println()
}

func (app *App) runREPL() error {
	// Create line editor
	editor := ui.NewLineEditor("> ", app.cmdRegistry.GetCompletions())

	for {
		outcome, err := editor.ReadLine()
		if err != nil {
			return fmt.Errorf("input error: %w", err)
		}

		if outcome.Exit {
			fmt.Println("\nGoodbye.")
			fmt.Println(app.costTracker.Summary())
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

		// Handle slash commands
		if result, handled, err := app.cmdRegistry.Handle(input); handled {
			if err != nil {
				fmt.Println(ui.RenderError(err.Error()))
				continue
			}
			if commands.IsQuit(result) {
				fmt.Println("\nGoodbye.")
				fmt.Println(app.costTracker.Summary())
				return nil
			}
			fmt.Println(result)
			continue
		}

		// Process user message through agent
		if err := app.processMessage(input); err != nil {
			fmt.Println(ui.RenderError(fmt.Sprintf("Error: %v", err)))
		}
	}
}

func (app *App) processMessage(input string) error {
	// Add user message to session
	userMsg := types.Message{
		UUID:      generateUUID(),
		Role:      types.RoleUser,
		Content:   []types.ContentBlock{types.TextBlock{Text: input}},
		Timestamp: time.Now(),
	}
	app.session.AddMessage(userMsg)

	// Build system prompt
	sysPrompt := app.buildSystemPrompt()

	// Create tool context
	toolCtx := tools.Context{
		Options: tools.Options{
			MainLoopModel: app.session.Model,
			Tools:         app.toolRegistry.FilterEnabled(),
			Debug:         false,
		},
		AbortController: context.Background(),
	}

	// Permission check function
	canUseTool := func(toolName string, toolInput map[string]any, ctx tools.Context) (tools.PermissionDecision, error) {
		t, ok := app.toolRegistry.FindToolByName(toolName)
		if !ok {
			return tools.PermissionDecision{Behavior: tools.Deny, Message: "unknown tool"}, nil
		}

		// Check permission mode
		switch app.config.PermissionMode {
		case config.PermissionReadOnly:
			// Only allow read/search tools
			if !isReadOnlyTool(toolName) {
				return tools.PermissionDecision{
					Behavior: tools.Deny,
					Message:  fmt.Sprintf("Permission denied: %s is not allowed in read-only mode", toolName),
				}, nil
			}
		case config.PermissionWorkspaceWrite:
			// Block dangerous tools
			if isDangerousTool(toolName) {
				return tools.PermissionDecision{
					Behavior: tools.Ask,
					Message:  fmt.Sprintf("This action requires confirmation: %s", toolName),
				}, nil
			}
		case config.PermissionDangerFullAccess:
			// Allow everything
		}

		// Check always allow/deny lists
		for _, allowed := range app.config.AlwaysAllow {
			if allowed == toolName {
				return tools.PermissionDecision{Behavior: tools.Allow}, nil
			}
		}
		for _, denied := range app.config.AlwaysDeny {
			if denied == toolName {
				return tools.PermissionDecision{Behavior: tools.Deny, Message: "tool is in deny list"}, nil
			}
		}

		// Use permission engine
		permCtx := permissions.EmptyContext()
		return permissions.Evaluate(t, toolInput, permCtx), nil
	}

	// Query the agent
	params := agent.QueryParams{
		Messages:       app.session.Messages,
		SystemPrompt:   sysPrompt,
		CanUseTool:     canUseTool,
		ToolUseContext: toolCtx,
	}

	stream, err := app.loop.Query(context.Background(), params)
	if err != nil {
		return err
	}

	// Process stream
	var responseMsg *types.Message
	for event := range stream {
		switch e := event.(type) {
		case types.StreamMessage:
			if responseMsg == nil {
				responseMsg = &e.Message
				app.renderMessage(e.Message)
			} else {
				// Append content
				responseMsg.Content = append(responseMsg.Content, e.Message.Content...)
				for _, block := range e.Message.Content {
					if text, ok := block.(types.TextBlock); ok {
						fmt.Print(text.Text)
					}
				}
			}
			app.session.AddMessage(e.Message)

		case types.ProgressMessage:
			fmt.Printf("\r[%s] %s", e.ToolUseID, e.Type)
		}
	}
	fmt.Println()

	// Complete the turn for cost tracking
	app.costTracker.CompleteTurn()

	// Auto-save session periodically
	if app.session.Turns%5 == 0 {
		if path, err := app.sessionManager.SaveCurrent(); err == nil {
			fmt.Printf(ui.DimStyle.Render("\n  [Session auto-saved: %s]\n"), filepath.Base(path))
		}
	}

	return nil
}

func (app *App) buildSystemPrompt() string {
	prompt := `You are Agent Harness, a helpful coding assistant.
You have access to tools for bash, file operations, search, and more.
Respect the user's workspace and permissions.
When editing files, ensure old_string matches exactly.
Use plan mode for complex multi-step tasks.`

	// Add git context if available
	if app.gitContext != nil && app.gitContext.IsRepo {
		prompt += fmt.Sprintf("\n\nWorking in git repository: %s", app.gitContext.Root)
		if app.gitContext.Branch != "" {
			prompt += fmt.Sprintf(" (branch: %s)", app.gitContext.Branch)
		}
	}

	// Load skills
	skillReg, _ := skills.LoadFromDirectory(".agent-harness/skills")
	for _, sk := range skillReg.All() {
		prompt += sk.FormatPrompt()
	}

	return prompt
}

func (app *App) renderMessage(msg types.Message) {
	for _, block := range msg.Content {
		switch b := block.(type) {
		case types.TextBlock:
			fmt.Print(b.Text)
		case types.ToolUseBlock:
			fmt.Printf("\n\n%s\n", ui.DimStyle.Render(fmt.Sprintf("[Using tool: %s]", b.Name)))
		}
	}
}

func (app *App) getStatusReport() string {
	meta := app.session.GetMetadata()
	
	projectRoot := ""
	gitBranch := ""
	if app.gitContext != nil && app.gitContext.IsRepo {
		projectRoot = app.gitContext.Root
		gitBranch = app.gitContext.Branch
	}

	return ui.RenderStatusReport(
		"active",
		meta.MessageCount,
		meta.Turns,
		meta.EstimatedTokens,
		app.session.Model,
		projectRoot,
		gitBranch,
	)
}

func (app *App) interactiveSetup(credManager *config.CredentialManager) error {
	fmt.Println()
	fmt.Println(ui.HeaderStyle.Render("╔════════════════════════════════════════════════════════════╗"))
	fmt.Println(ui.HeaderStyle.Render("║     Agent Harness - Secure Initial Setup                   ║"))
	fmt.Println(ui.HeaderStyle.Render("╚════════════════════════════════════════════════════════════╝"))
	fmt.Println()
	fmt.Println("No API credentials found. Let's set them up securely.")
	fmt.Println()

	// Provider selection (simplified)
	fmt.Println("Choose an API provider:")
	fmt.Println("  1) OpenRouter (recommended)")
	fmt.Println("  2) OpenAI")
	fmt.Println("  3) Anthropic")
	fmt.Print("Enter choice (1-3) [1]: ")

	var choice string
	fmt.Scanln(&choice)

	switch choice {
	case "2":
		app.config.Provider = "openai"
	case "3":
		app.config.Provider = "anthropic"
	default:
		app.config.Provider = "openrouter"
	}

	fmt.Println()
	fmt.Printf("Selected: %s\n", app.config.Provider)
	fmt.Println()

	// API key input with masking
	fmt.Printf("Enter your %s API key: ", app.config.Provider)
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

	// Model selection
	defaultModel := "anthropic/claude-3.5-sonnet"
	if app.config.Provider == "openai" {
		defaultModel = "gpt-4o"
	} else if app.config.Provider == "anthropic" {
		defaultModel = "claude-3-5-sonnet-20241022"
	}

	fmt.Printf("Model [%s]: ", defaultModel)
	var model string
	fmt.Scanln(&model)
	if model != "" {
		app.config.Model = model
	} else {
		app.config.Model = defaultModel
	}

	fmt.Println()
	fmt.Println("Credentials will be encrypted with a master password.")
	fmt.Println("You'll need this password each time you start agent-harness.")
	fmt.Println()

	// Save securely
	secureCfg := &config.SecureConfig{
		Provider: app.config.Provider,
		APIKey:   app.config.APIKey,
		Model:    app.config.Model,
	}

	if err := credManager.SaveSecure(secureCfg); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	fmt.Println()
	fmt.Println(ui.HeaderStyle.Render("╔════════════════════════════════════════════════════════════╗"))
	fmt.Println(ui.HeaderStyle.Render("║  " + ui.RenderSuccess("Credentials saved securely") + "                              ║"))
	fmt.Println(ui.HeaderStyle.Render("║                                                            ║"))
	fmt.Println(ui.HeaderStyle.Render("║  Encryption: AES-256-GCM with Argon2id                     ║"))
	fmt.Println(ui.HeaderStyle.Render("║  File permissions: 0600 (user read/write only)             ║"))
	fmt.Println(ui.HeaderStyle.Render("╚════════════════════════════════════════════════════════════╝"))
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
	// Simple UUID generation - in production use proper UUID library
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
