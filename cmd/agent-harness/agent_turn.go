package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/agents"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/diag"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
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

// handleAgentLoopAsync runs the full agent loop asynchronously.
//
// The turn runs on its own goroutine — outside the TUI's Update/View
// recover nets — so a panic here kills the whole program (bubbletea
// catches goroutine panics and exits). A turn that panics must cost
// the turn, not the session: the recover degrades the panic to an
// error message in the transcript and a diagnostics entry.
func (app *App) handleAgentLoopAsync(input string, tuiApp *tui.App) {
	defer func() {
		if r := recover(); r != nil {
			diag.Panic("agent.turn.panic", r)
			tuiApp.Send(tui.AgentErrorMsg{
				Error:     fmt.Errorf("internal error recovered (site: agent.turn.panic). Trace: ~/.agent-harness/logs"),
				Timestamp: time.Now(),
			})
		}
	}()
	app.runAgentTurn(input, tuiApp)
}

func (app *App) runAgentTurn(input string, tuiApp *tui.App) {
	// PRE-FLIGHT: Check common config issues before calling LLM
	if err := app.validateConfig(); err != nil {
		tuiApp.Send(tui.AgentErrorMsg{Error: err, Timestamp: time.Now()})
		return
	}

	tuiApp.Send(tui.AgentStartMsg{Timestamp: time.Now()})
	turnStarted := time.Now()
	// Show connecting state so user knows something is happening
	tuiApp.Send(tui.AgentConnectingMsg{Endpoint: app.config.Provider})

	sysPrompt, turnTools, maxOutput := app.turnProfile(input)
	promptTokens := agent.EstimatePromptTokens(sysPrompt, turnTools)
	maxOutput = fitOutputBudget(app.config.ContextLength, promptTokens, app.session.EstimateTokens(), maxOutput)
	diag.Infof("agent.turn.request", "provider=%s model=%s system_chars=%d tools=%d prompt_tokens=%d max_output=%d", app.config.Provider, app.session.Model, len(sysPrompt), len(turnTools), promptTokens, maxOutput)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	beginApprovalTurn(app, ctx)
	defer endApprovalTurn(app)

	tuiApp.SetAgentCancelFunc(cancel)
	defer tuiApp.SetAgentCancelFunc(nil)
	canUseTool := app.createToolPermissionFunc(tuiApp)

	toolCtx := tools.Context{
		Options: tools.Options{
			MainLoopModel: app.session.Model,
			Tools:         turnTools,
			Debug:         false,
		},
		AbortController:   ctx,
		RequireCanUseTool: true,
	}
	toolCtx.SubAgentQuery = func(prompt string, agentType string) (string, error) {
		return app.runDelegatedAgent(ctx, prompt, agentType, toolCtx, canUseTool)
	}
	if tuiApp != nil {
		toolCtx.AskUser = app.requestUserQuestion
	}

	params := agent.QueryParams{
		Messages:        app.session.Messages,
		SystemPrompt:    sysPrompt,
		CanUseTool:      canUseTool,
		ToolUseContext:  toolCtx,
		MaxOutputTokens: maxOutput,
		Temperature:     app.config.Temperature,
		TemperatureSet:  app.config.TemperatureSet,
		ReasoningEffort: app.config.Effort,
		// The session-scoped /limit knob overrides the loop default.
		MaxToolCalls: app.session.ToolLimit,
		PromptTokens: promptTokens,
	}

	stream, err := app.loop.Query(ctx, params)
	if err != nil {
		tuiApp.Send(tui.AgentErrorMsg{Error: err, Timestamp: time.Now()})
		return
	}

	var responseText strings.Builder
	// requestText is the prose streamed for the current provider
	// request. The final StreamMessage repeats that prose; comparing
	// the two lets the TUI render deltas without duplicating the
	// assembled message, while still falling back for providers that
	// never emit deltas.
	var requestText strings.Builder
	// announcedTools carries calls already reported by the transient
	// StreamToolUse event, so the final message does not open a second
	// row for the same call.
	announcedTools := map[string]bool{}
	toolCallCount := 0
	var persistenceErr error
	var terminal *types.StreamTerminal
	settlementSent := false
	firstOutput := false

	for event := range stream {
		// Keep draining after a persistence failure so the producer can close
		// cleanly, but do not apply or report later events as a successful turn.
		if persistenceErr != nil {
			continue
		}

		switch e := event.(type) {
		case types.StreamRequestStart:
			requestText.Reset()
		case types.StreamTextDelta:
			if e.Text == "" {
				break
			}
			if !firstOutput && strings.TrimSpace(e.Text) != "" {
				firstOutput = true
				diag.Infof("agent.turn.first_output", "first text after %s", time.Since(turnStarted).Round(time.Millisecond))
			}
			tuiApp.Send(tui.AgentChunkMsg{Text: e.Text, Timestamp: time.Now()})
			responseText.WriteString(e.Text)
			requestText.WriteString(e.Text)
		case types.StreamToolUse:
			if !announcedTools[e.Block.ID] {
				app.handleToolUseStart(e.Block, tuiApp)
				announcedTools[e.Block.ID] = true
			}
		case types.ProgressMessage:
			// Progress is temporary tool activity, never model speech or
			// session evidence. Strip terminal controls from server messages.
			text := strings.Join(strings.Fields(tui.SanitizeANSI(fmt.Sprint(e.Data))), " ")
			tuiApp.Send(tui.StatusMsg{Text: text, Type: "info"})
		case types.StreamContextCompacted:
			// Persist the exact message snapshot used by subsequent model
			// requests without replacing persona, plan mode, or session identity.
			app.session.Messages = append([]types.Message(nil), e.Messages...)
			app.session.UpdatedAt = time.Now()
			app.session.Version++
			app.sessionManager.SetCurrent(app.session)
			if _, err := app.sessionManager.SaveCurrent(); err != nil {
				persistenceErr = fmt.Errorf("persist compacted session: %w", err)
				cancel()
				tuiApp.Send(tui.AgentErrorMsg{
					Error:     persistenceErr,
					Timestamp: time.Now(),
				})
			} else if e.Notice != "" {
				tuiApp.Send(tui.StatusMsg{Text: e.Notice, Type: "info"})
			}
		case types.StreamThinking:
			// Reasoning preview: update the badge text without touching
			// the thinking timer (SetThinking would reset the clock on
			// every reasoning delta). Rides the channel: the chat model
			// must only be touched on the event loop.
			tuiApp.Send(tui.AgentThinkingMsg{Text: e.Text})
		case types.StreamMessage:
			// System-role notices (tool-call limit, loop detection) are
			// loop announcements, not model speech: they must render as
			// system messages. Streaming them as AgentChunkMsg made a
			// fake assistant bubble out of "[Tool loop detected...]".
			if e.Message.Role == types.RoleSystem {
				for _, block := range e.Message.Content {
					if tb, ok := block.(types.TextBlock); ok && tb.Text != "" {
						tuiApp.Send(tui.AgentSystemNoteMsg{Text: tb.Text})
					}
				}
				break
			}
			alreadyStreamed := streamedTextMatches(e.Message, requestText.String())
			for _, block := range e.Message.Content {
				switch b := block.(type) {
				case types.TextBlock:
					if alreadyStreamed {
						break
					}
					if !firstOutput && strings.TrimSpace(b.Text) != "" {
						firstOutput = true
						diag.Infof("agent.turn.first_output", "first text after %s", time.Since(turnStarted).Round(time.Millisecond))
					}
					tuiApp.Send(tui.AgentChunkMsg{
						Text:      b.Text,
						Timestamp: time.Now(),
					})
					responseText.WriteString(b.Text)
				case types.ToolUseBlock:
					toolCallCount++
					if !announcedTools[b.ID] {
						app.handleToolUseStart(b, tuiApp)
						announcedTools[b.ID] = true
					}
				case types.ToolResultBlock:
					tuiApp.Send(tui.AgentToolDoneMsg{
						ToolID:  b.ToolUseID,
						Success: !b.IsError,
						Output:  fmt.Sprintf("%v", b.Content),
					})
				}
			}
			app.session.AddMessage(e.Message)
			app.sessionManager.SetCurrent(app.session)
			if _, err := app.sessionManager.SaveCurrent(); err != nil {
				diag.Error("session.save.turn", err)
			}
		case types.StreamError:
			// Esc cancels the turn context; the loop reports that as a
			// provider error. The TUI already owns the cancellation notice,
			// so rendering "Error: context canceled" on top of it reads as a
			// second, contradictory failure.
			if userCancelled(ctx) {
				break
			}
			// The pane shows the actionable summary; the Logs tab keeps
			// the full provider sentence and limit metadata for diagnosis.
			diag.Errorf("provider.stream", "%v", e.Error)
			if !settlementSent {
				tuiApp.Send(tui.AgentErrorMsg{Error: e.Error, Timestamp: time.Now()})
				settlementSent = true
			}
		case types.StreamTerminal:
			terminal = &e
		}
	}

	if persistenceErr != nil {
		return
	}
	if userCancelled(ctx) {
		diag.Infof("agent.turn.cancelled", "cancelled by user after %s", time.Since(turnStarted).Round(100*time.Millisecond))
		return
	}
	if terminal == nil {
		terminal = &types.StreamTerminal{Reason: string(agent.TerminalReasonError), Error: fmt.Errorf("agent stream ended without a terminal event")}
	}
	if terminal.Reason != string(agent.TerminalReasonComplete) {
		if !settlementSent {
			err := terminal.Error
			if err == nil {
				err = fmt.Errorf("agent turn ended: %s", terminal.Reason)
			}
			diag.Errorf("provider.turn", "%v", err)
			tuiApp.Send(tui.AgentErrorMsg{Error: err, Timestamp: time.Now()})
		}
		return
	}

	// Feed provider-reported usage into the cost tracker, then close the turn.
	if app.loop != nil {
		usage := app.loop.LastUsage
		app.costTracker.AddToCurrentTurn(agent.TokenUsage{
			InputTokens:              usage.InputTokens,
			OutputTokens:             usage.OutputTokens,
			CacheReadInputTokens:     usage.CacheReadInputTokens,
			CacheCreationInputTokens: usage.CacheCreationInputTokens,
		})
	}
	app.costTracker.CompleteTurn()
	app.refreshTelemetry(tuiApp)

	tuiApp.Send(tui.AgentDoneMsg{
		FullResponse: responseText.String(),
		ToolCalls:    toolCallCount,
		Timestamp:    time.Now(),
	})

	// Turn lifecycle lands in the diagnostics stream: one INFO line per
	// completed turn is the spine of the Logs tab's story.
	diag.Infof("agent.turn.complete", "turn finished in %s (%d tool calls, %d chars)",
		time.Since(turnStarted).Round(100*time.Millisecond), toolCallCount, responseText.Len())

	// Auto-save check: notices land in the chat pane + Settings system
	// log (deduped, once), never in the footer.
	if app.session.Turns%5 == 0 {
		if path, err := app.sessionManager.SaveCurrent(); err == nil {
			tuiApp.Send(tui.SessionsRefreshedMsg{
				Sessions:   app.getSessionInfos(),
				Notice:     sprintf("Auto-saved to %s", path),
				NoticeType: "info",
			})
		}
	}
}

// userCancelled reports the Esc path: the turn context was canceled by
// the user, not by a provider deadline or an internal failure.
func userCancelled(ctx context.Context) bool {
	return errors.Is(ctx.Err(), context.Canceled)
}

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
	stream, err := childLoop.Query(subCtx, agent.QueryParams{
		Messages:        []types.Message{{UUID: generateUUID(), Role: types.RoleUser, Content: []types.ContentBlock{types.TextBlock{Text: prompt}}, Timestamp: time.Now()}},
		SystemPrompt:    childSystemPrompt,
		CanUseTool:      canUseTool,
		ToolUseContext:  childCtx,
		MaxOutputTokens: app.config.MaxTokens,
		Temperature:     app.config.Temperature,
		TemperatureSet:  app.config.TemperatureSet,
		MaxTurns:        delegatedAgentMaxTurns,
		MaxToolCalls:    delegatedAgentMaxTools,
		PromptTokens:    agent.EstimatePromptTokens(childSystemPrompt, childTools),
	})
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

// createToolPermissionFunc creates the permission checking function for tools.
