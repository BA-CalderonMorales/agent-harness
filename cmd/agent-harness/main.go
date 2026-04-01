package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/permissions"
	"github.com/BA-CalderonMorales/agent-harness/internal/tools"
	"github.com/BA-CalderonMorales/agent-harness/internal/tools/builtin"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"github.com/google/uuid"
)

var (
	// Version is injected at build time.
	Version = "dev"
)

func main() {
	cfg := config.Load()
	if cfg.APIKey == "" {
		fmt.Fprintln(os.Stderr, "Error: Set OPENROUTER_API_KEY or ANTHROPIC_API_KEY")
		os.Exit(1)
	}

	fmt.Printf("agent-harness %s\nModel: %s\nType 'exit' to quit.\n\n", Version, cfg.Model)

	client := llm.NewHTTPClient(cfg.Provider, cfg.APIKey)
	loop := agent.NewLoop(client)

	registry := tools.NewRegistry()
	registry.RegisterBuiltIn(builtin.BashTool)
	registry.RegisterBuiltIn(builtin.FileReadTool)
	registry.RegisterBuiltIn(builtin.FileEditTool)

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
		if input == "exit" || input == "quit" {
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

		params := agent.QueryParams{
			Messages:       messages,
			SystemPrompt:   buildSystemPrompt(),
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
You have access to tools: bash, read, edit.
When editing files, ensure old_string matches exactly.
When running bash commands, respect the user's system.`
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
