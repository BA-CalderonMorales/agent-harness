package main

import (
	"context"
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/agents"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools/builtin"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"strings"
	"time"
)

const (
	delegatedAgentTimeout  = 60 * time.Second
	delegatedAgentMaxTurns = 4
	delegatedAgentMaxTools = 8
)

// runDelegatedAgent executes a child with fresh messages and bounded runtime.
// The parent's context and permission callback are deliberately retained: a
// child cannot outlive its request or bypass the user's tool decisions.
func (app *App) runDelegatedAgent(parent context.Context, prompt string, agentType string, parentToolCtx tools.Context, canUseTool tools.CanUseToolFn) (string, error) {
	subCtx, subCancel := context.WithTimeout(parent, delegatedAgentTimeout)
	defer subCancel()

	// A named sub-agent comes from a definition file: its own
	// instructions, its own tool set, its own model. An unknown name is
	// an error rather than a silent generic worker, so the model can
	// correct itself instead of thinking it delegated to a specialist.
	definition, hasDefinition := agents.Definition{}, false
	if agentType != "" {
		registry := app.loadAgentDefs()
		def, ok := registry.Get(agentType)
		if !ok {
			names := make([]string, 0, registry.Len())
			for _, def := range registry.All() {
				names = append(names, def.Name)
			}
			if len(names) == 0 {
				return "", fmt.Errorf("unknown sub-agent type %q: no agent definitions are installed", agentType)
			}
			return "", fmt.Errorf("unknown sub-agent type %q (available: %s)", agentType, strings.Join(names, ", "))
		}
		definition, hasDefinition = def, true
	}

	allowed := map[string]bool{}
	if hasDefinition && len(definition.Tools) > 0 {
		for _, name := range definition.Tools {
			allowed[strings.ToLower(name)] = true
		}
	}
	childTools := make([]tools.Tool, 0, len(parentToolCtx.Options.Tools))
	for _, tool := range parentToolCtx.Options.Tools {
		if tool.Name == builtin.AgentTool.Name {
			continue
		}
		if len(allowed) > 0 && !allowed[strings.ToLower(tool.Name)] {
			continue
		}
		childTools = append(childTools, tool)
	}
	childModel := app.session.Model
	if hasDefinition && definition.Model != "" && !strings.EqualFold(definition.Model, "inherit") {
		childModel = definition.Model
	}
	childCtx := tools.Context{
		Options: tools.Options{
			MainLoopModel: childModel,
			Tools:         childTools,
			Debug:         false,
		},
		AbortController:   subCtx,
		QueryTracking:     tools.QueryChainTracking{ChainID: parentToolCtx.QueryTracking.ChainID, Depth: parentToolCtx.QueryTracking.Depth + 1},
		RequireCanUseTool: true,
	}
	if app.tuiApp != nil {
		childCtx.AskUser = app.requestUserQuestion
	}
	childLoop := agent.NewLoop(app.client)
	childLoop.Config.DefaultMaxTurns = delegatedAgentMaxTurns
	childLoop.Config.MaxToolCalls = delegatedAgentMaxTools
	childLoop.Config.StreamingToolExecution = false
	childSystemPrompt := delegatedSystemPrompt(app.buildSystemPrompt(), definition, hasDefinition)
	stream, err := childLoop.Query(subCtx, app.delegatedQueryParams(prompt, childSystemPrompt, childTools, canUseTool, childCtx))
	if err != nil {
		return "", fmt.Errorf("delegated worker failed: %w", err)
	}
	var result strings.Builder
	var terminal types.StreamTerminal
	terminalSeen := false
	for event := range stream {
		switch e := event.(type) {
		case types.StreamMessage:
			for _, block := range e.Message.Content {
				if text, ok := block.(types.TextBlock); ok {
					result.WriteString(text.Text)
				}
			}
		case types.StreamError:
			if e.Error != nil {
				return result.String(), fmt.Errorf("delegated worker failed: %w", e.Error)
			}
		case types.StreamTerminal:
			terminal, terminalSeen = e, true
		}
	}
	if !terminalSeen {
		return result.String(), fmt.Errorf("delegated worker ended without a terminal result")
	}
	if terminal.Reason != string(agent.TerminalReasonComplete) {
		if terminal.Error != nil {
			return result.String(), fmt.Errorf("delegated worker failed (%s): %w", terminal.Reason, terminal.Error)
		}
		return result.String(), fmt.Errorf("delegated worker stopped before completion: %s", terminal.Reason)
	}
	return result.String(), nil
}

// delegatedQueryParams is the query a sub-agent runs. It carries the same
// record compaction lab choice as the turn that started the sub-agent.
func (app *App) delegatedQueryParams(prompt, systemPrompt string, childTools []tools.Tool, canUseTool tools.CanUseToolFn, childCtx tools.Context) agent.QueryParams {
	return agent.QueryParams{
		Messages:         []types.Message{{UUID: generateUUID(), Role: types.RoleUser, Content: []types.ContentBlock{types.TextBlock{Text: prompt}}, Timestamp: time.Now()}},
		SystemPrompt:     systemPrompt,
		CanUseTool:       canUseTool,
		ToolUseContext:   childCtx,
		MaxOutputTokens:  app.config.MaxTokens,
		Temperature:      app.config.Temperature,
		TemperatureSet:   app.config.TemperatureSet,
		MaxTurns:         delegatedAgentMaxTurns,
		MaxToolCalls:     delegatedAgentMaxTools,
		PromptTokens:     agent.EstimatePromptTokens(systemPrompt, childTools),
		RecordCompaction: app.config.LabEnabled(config.LabRecordCompaction),
	}
}
