package main

import (
	"context"
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"strings"
	"time"
)

// handleAgentLoopAsync runs the full agent loop asynchronously.
func (app *App) handleAgentLoopAsync(input string, tuiApp *tui.App) {
	// PRE-FLIGHT: Check common config issues before calling LLM
	if err := app.validateConfig(); err != nil {
		tuiApp.Send(tui.AgentErrorMsg{Error: err, Timestamp: time.Now()})
		return
	}

	tuiApp.Send(tui.AgentStartMsg{Timestamp: time.Now()})
	// Show connecting state so user knows something is happening
	tuiApp.Send(tui.AgentConnectingMsg{Endpoint: app.config.Provider})

	sysPrompt := app.buildSystemPrompt()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tuiApp.SetAgentCancelFunc(cancel)
	defer tuiApp.SetAgentCancelFunc(nil)

	toolCtx := tools.Context{
		Options: tools.Options{
			MainLoopModel: app.session.Model,
			Tools:         app.toolRegistry.FilterEnabled(),
			Debug:         false,
		},
		AbortController:   ctx,
		RequireCanUseTool: true,
		SubAgentQuery: func(prompt string) (string, error) {
			// Sub-agent runs a single-turn query with fresh context
			subCtx, subCancel := context.WithTimeout(ctx, 60*time.Second)
			defer subCancel()
			req := llm.Request{
				Messages: []types.Message{
					{UUID: generateUUID(), Role: types.RoleUser, Content: []types.ContentBlock{types.TextBlock{Text: prompt}}, Timestamp: time.Now()},
				},
				SystemPrompt: app.buildSystemPrompt(),
				Model:        app.session.Model,
				MaxTokens:    app.config.MaxTokens,
				Temperature:  app.config.Temperature,
			}
			stream, err := app.client.Stream(subCtx, req)
			if err != nil {
				return "", err
			}
			var result strings.Builder
			for event := range stream {
				switch e := event.(type) {
				case types.LLMTextDelta:
					result.WriteString(e.Delta)
				case types.LLMMessageStop:
					// done
				case types.LLMError:
					return result.String(), e.Error
				}
			}
			return result.String(), nil
		},
	}

	canUseTool := app.createToolPermissionFunc(tuiApp)

	params := agent.QueryParams{
		Messages:        app.session.Messages,
		SystemPrompt:    sysPrompt,
		CanUseTool:      canUseTool,
		ToolUseContext:  toolCtx,
		MaxOutputTokens: app.config.MaxTokens,
		Temperature:     app.config.Temperature,
		ReasoningEffort: app.config.Effort,
		// The session-scoped /limit knob overrides the loop default.
		MaxToolCalls: app.session.ToolLimit,
	}

	stream, err := app.loop.Query(ctx, params)
	if err != nil {
		tuiApp.Send(tui.AgentErrorMsg{Error: err, Timestamp: time.Now()})
		return
	}

	var responseText strings.Builder
	toolCallCount := 0
	var persistenceErr error

	for event := range stream {
		// Keep draining after a persistence failure so the producer can close
		// cleanly, but do not apply or report later events as a successful turn.
		if persistenceErr != nil {
			continue
		}

		switch e := event.(type) {
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
		case types.StreamMessage:
			// System-role notices (tool-call limit, loop detection) are
			// loop announcements, not model speech: they must render as
			// system messages. Streaming them as AgentChunkMsg made a
			// fake assistant bubble out of "[Tool loop detected...]".
			if e.Message.Role == types.RoleSystem {
				for _, block := range e.Message.Content {
					if tb, ok := block.(types.TextBlock); ok && tb.Text != "" {
						tuiApp.AddMessage("system", tb.Text)
					}
				}
				break
			}
			for _, block := range e.Message.Content {
				switch b := block.(type) {
				case types.TextBlock:
					tuiApp.Send(tui.AgentChunkMsg{
						Text:      b.Text,
						Timestamp: time.Now(),
					})
					responseText.WriteString(b.Text)
				case types.ToolUseBlock:
					toolCallCount++
					app.handleToolUseStart(b, tuiApp)
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
			_, _ = app.sessionManager.SaveCurrent()
		case types.StreamError:
			tuiApp.Send(tui.AgentErrorMsg{Error: e.Error, Timestamp: time.Now()})
		}
	}

	if persistenceErr != nil {
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

// createToolPermissionFunc creates the permission checking function for tools.
