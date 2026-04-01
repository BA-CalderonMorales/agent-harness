package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/commands"
	"github.com/BA-CalderonMorales/agent-harness/internal/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/permissions"
	"github.com/BA-CalderonMorales/agent-harness/internal/skills"
	"github.com/BA-CalderonMorales/agent-harness/internal/tools"
	"github.com/BA-CalderonMorales/agent-harness/internal/tools/builtin"
	"github.com/BA-CalderonMorales/agent-harness/internal/ui"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"github.com/google/uuid"
)

var (
	// Version is injected at build time.
	Version = "dev"
)

func main() {
	useTUI := len(os.Args) > 1 && os.Args[1] == "--tui"

	cfg := config.Load()
	fileCfg, _ := config.LoadFile(config.DefaultConfigPath())
	if fileCfg.Provider != "" {
		cfg.Provider = fileCfg.Provider
	}
	if fileCfg.Model != "" {
		cfg.Model = fileCfg.Model
	}

	if cfg.APIKey == "" {
		fmt.Fprintln(os.Stderr, "Error: Set OPENROUTER_API_KEY or ANTHROPIC_API_KEY")
		os.Exit(1)
	}

	if useTUI {
		runTUI(cfg)
		return
	}

	fmt.Printf("agent-harness %s\nModel: %s\nType 'exit' to quit.\n\n", Version, cfg.Model)

	client := llm.NewHTTPClient(cfg.Provider, cfg.APIKey)
	loop := agent.NewLoop(client)
	costTracker := agent.NewCostTracker()

	registry := tools.NewRegistry()
	registry.RegisterBuiltIn(builtin.BashTool)
	registry.RegisterBuiltIn(builtin.FileReadTool)
	registry.RegisterBuiltIn(builtin.FileEditTool)
	registry.RegisterBuiltIn(builtin.FileWriteTool)
	registry.RegisterBuiltIn(builtin.NotebookEditTool)
	registry.RegisterBuiltIn(builtin.RewindTool)
	registry.RegisterBuiltIn(builtin.GlobTool)
	registry.RegisterBuiltIn(builtin.GrepTool)
	registry.RegisterBuiltIn(builtin.AskUserQuestionTool)
	registry.RegisterBuiltIn(builtin.TodoWriteTool)
	registry.RegisterBuiltIn(builtin.AgentTool)
	registry.RegisterBuiltIn(builtin.WebFetchTool)
	registry.RegisterBuiltIn(builtin.WebSearchTool)
	registry.RegisterBuiltIn(builtin.EnterPlanModeTool)
	registry.RegisterBuiltIn(builtin.ExitPlanModeTool)
	registry.RegisterBuiltIn(builtin.ExportTool)
	registry.RegisterBuiltIn(builtin.SearchTranscriptTool)
	registry.RegisterBuiltIn(builtin.SettingsTool)

	cmdRegistry := commands.NewRegistry()
	for _, cmd := range commands.BuiltInCommands() {
		cmdRegistry.Register(cmd)
	}

	permCtx := permissions.EmptyContext()
	canUseTool := func(toolName string, input map[string]any, ctx tools.Context) (tools.PermissionDecision, error) {
		t, ok := registry.FindToolByName(toolName)
		if !ok {
			return tools.PermissionDecision{Behavior: tools.Deny, Message: "unknown tool"}, nil
		}
		decision := permissions.Evaluate(t, input, permCtx)
		if decision.Behavior == tools.Ask {
			fmt.Printf("\n[Permission] Allow %s? (y/n): ", toolName)
			reader := bufio.NewReader(os.Stdin)
			resp, _ := reader.ReadString('\n')
			resp = strings.TrimSpace(strings.ToLower(resp))
			if resp == "y" || resp == "yes" {
				decision.Behavior = tools.Allow
			} else {
				decision.Behavior = tools.Deny
			}
		}
		return decision, nil
	}

	var messages []types.Message
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Handle slash commands
		if cmdName, args, ok := commands.Parse(input); ok {
			cmd, found := cmdRegistry.Find(cmdName)
			if found {
				result, cmdErr := cmd.Handler(args)
				if cmdErr != nil {
					fmt.Printf("Error: %v\n", cmdErr)
					continue
				}
				switch result {
				case "history_cleared":
					messages = nil
					fmt.Println("History cleared.")
				case "compact_triggered":
					fmt.Println("Compact triggered (placeholder).")
				case "exit_requested":
					fmt.Println("Goodbye.")
					fmt.Println(costTracker.Summary())
					return
				case "help_displayed":
					fmt.Println("Available commands:")
					for _, c := range cmdRegistry.All() {
						fmt.Printf("  /%-12s %s\n", c.Name, c.Description)
					}
				default:
					if strings.HasPrefix(result, "model_changed:") {
						cfg.Model = strings.TrimPrefix(result, "model_changed:")
						fmt.Printf("Model changed to %s\n", cfg.Model)
					}
				}
				continue
			}
			fmt.Printf("Unknown command: /%s\n", cmdName)
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye.")
			fmt.Println(costTracker.Summary())
			break
		}

		userMsg := types.Message{
			UUID:      uuid.New().String(),
			Role:      types.RoleUser,
			Content:   []types.ContentBlock{types.TextBlock{Text: input}},
			Timestamp: time.Now(),
		}
		messages = append(messages, userMsg)

		toolCtx := tools.Context{
			Options: tools.Options{
				MainLoopModel: cfg.Model,
				Tools:         registry.FilterEnabled(),
				Debug:         cfg.Verbose,
			},
			AbortController: context.Background(),
		}

		// Load project memory files
		memoryLoader := agent.NewMemoryLoader()
		memories, _ := memoryLoader.LoadForDirectory(cfg.WorkingDirectory)
		sysPrompt := buildSystemPrompt()
		for _, mem := range memories {
			sysPrompt += mem.FormatSystemPrompt()
		}

		// Load skills
		skillReg, _ := skills.LoadFromDirectory(".agent-harness/skills")
		for _, sk := range skillReg.All() {
			sysPrompt += sk.FormatPrompt()
		}

		params := agent.QueryParams{
			Messages:       messages,
			SystemPrompt:   sysPrompt,
			CanUseTool:     canUseTool,
			ToolUseContext: toolCtx,
		}

		stream, err := loop.Query(context.Background(), params)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		for event := range stream {
			switch e := event.(type) {
			case types.StreamMessage:
				renderMessage(e.Message)
				messages = append(messages, e.Message)

			case types.ProgressMessage:
				fmt.Printf("\r[%s] %s\n", e.ToolUseID, e.Type)
			case types.StreamRequestStart:
				// Silently track start
			}
		}
		fmt.Println()
	}
}

func buildSystemPrompt() string {
	return `You are Agent Harness, a helpful coding assistant.
You have access to tools: bash, read, write, edit, notebook_edit, rewind, glob, grep, ask_user_question, todo_write, agent, web_fetch, web_search, search_transcript, export, settings, enter_plan_mode, exit_plan_mode.
When editing files, ensure old_string matches exactly.
When running bash commands, respect the user's system.
Use plan mode for complex multi-step tasks.
Use rewind to undo file operations if needed.`
}

func renderMessage(msg types.Message) {
	for _, block := range msg.Content {
		switch b := block.(type) {
		case types.TextBlock:
			fmt.Print(b.Text)
		case types.ToolUseBlock:
			fmt.Printf("\n[Using tool: %s]\n", b.Name)
		}
	}
}

func runTUI(cfg config.Config) {
	fmt.Println("TUI mode not yet fully implemented. Run without --tui for CLI mode.")
	_ = ui.NewModel(nil, nil)
}
