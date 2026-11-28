package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/diag"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// maxToolLimit caps the session /limit knob: the bump is a rescope for a
// long task, not a runaway jailbreak - the convergence guard
// (MaxIdenticalToolUses) stays the real backstop regardless.
const maxToolLimit = 500

// toolCallSignature canonicalizes a tool use for loop detection: name plus
// the JSON of the input (Go serializes map keys deterministically).
func toolCallSignature(tu types.ToolUseBlock) string {
	in, err := json.Marshal(tu.Input)
	if err != nil {
		in = []byte(fmt.Sprintf("%v", tu.Input))
	}
	return tu.Name + "|" + string(in)
}

// NewLoop creates an agent loop with the given LLM client.
func NewLoop(client llm.Client) *Loop {
	return &Loop{
		Client: client,
		Config: DefaultLoopConfig(),
	}
}

// LastTokenUsage returns the accumulated token usage for the latest query.
func (l *Loop) LastTokenUsage() types.TokenUsage {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.LastUsage
}

func (l *Loop) resetLastUsage() {
	l.mu.Lock()
	l.LastUsage = types.TokenUsage{}
	l.mu.Unlock()
}

func (l *Loop) addUsage(usage types.TokenUsage) {
	l.mu.Lock()
	l.LastUsage.InputTokens += usage.InputTokens
	l.LastUsage.OutputTokens += usage.OutputTokens
	l.LastUsage.CacheCreationInputTokens += usage.CacheCreationInputTokens
	l.LastUsage.CacheReadInputTokens += usage.CacheReadInputTokens
	l.mu.Unlock()
}

// buildLLMRequest builds a provider request with deterministic SystemContext
// ordering (preserving prompt caches) and consistent parameters across both
// primary turns and recovery retries.
func buildLLMRequest(params QueryParams, state *loopState) llm.Request {
	sysPrompt := params.SystemPrompt
	if len(params.SystemContext) > 0 {
		keys := make([]string, 0, len(params.SystemContext))
		for k := range params.SystemContext {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var b strings.Builder
		b.WriteString(sysPrompt)
		for _, k := range keys {
			fmt.Fprintf(&b, "\n\n<%s>\n%s\n</%s>", k, params.SystemContext[k], k)
		}
		sysPrompt = b.String()
	}

	model := params.ToolUseContext.Options.MainLoopModel
	if model == "" {
		model = config.DefaultModel
	}

	maxTokens := 8192
	if state.maxOutputTokensOverride > 0 {
		maxTokens = state.maxOutputTokensOverride
	}

	return llm.Request{
		Messages:        state.messages,
		SystemPrompt:    sysPrompt,
		Tools:           params.ToolUseContext.Options.Tools,
		Model:           model,
		MaxTokens:       maxTokens,
		Temperature:     params.Temperature,
		TemperatureSet:  params.TemperatureSet,
		ReasoningEffort: params.ReasoningEffort,
	}
}

// Query runs the full agent loop for a single user turn.
// It yields StreamEvents so the caller can render progress in real time.
//
// The loop's goroutine carries its own recover net: a panic anywhere in
// the turn (a tool executor bug, a race spike, a provider parser blowup)
// becomes a StreamError the caller renders as a failed turn. Without it
// the panic reaches bubbletea's goroutine handler and the whole TUI dies.
func (l *Loop) Query(ctx context.Context, params QueryParams) (<-chan types.StreamEvent, error) {
	out := make(chan types.StreamEvent, 16)
	l.resetLastUsage()

	go func() {
		defer close(out)
		defer func() {
			if r := recover(); r != nil {
				diag.Panic("agent.loop.panic", r)
				out <- types.StreamError{
					Error: fmt.Errorf("internal error recovered (site: agent.loop.panic). Trace: ~/.agent-harness/logs"),
				}
			}
		}()
		state := loopState{
			messages:                     params.Messages,
			promptTokens:                 params.PromptTokens,
			record:                       seedRecord(params),
			recordOff:                    !params.RecordCompaction,
			toolUseContext:               params.ToolUseContext,
			maxOutputTokensOverride:      params.MaxOutputTokens,
			maxOutputTokensRecoveryCount: 0,
			turnCount:                    1,
			executedTools:                make(map[string]int),
		}

		terminal := l.queryLoop(ctx, params, &state, out)
		out <- types.StreamTerminal{
			Reason:  string(terminal.Reason),
			Message: terminal.Message,
			Error:   terminal.Error,
		}
	}()

	return out, nil
}

// queryLoop is the while-true agent loop.
func (l *Loop) queryLoop(ctx context.Context, params QueryParams, state *loopState, out chan<- types.StreamEvent) Terminal {
	if state.executedTools == nil {
		state.executedTools = make(map[string]int)
	}
	if params.MaxOutputTokens > 0 && state.maxOutputTokensOverride == 0 {
		state.maxOutputTokensOverride = params.MaxOutputTokens
	}

	maxTurns := params.MaxTurns
	if maxTurns == 0 {
		maxTurns = l.Config.DefaultMaxTurns
	}

	for state.turnCount <= maxTurns {
		state.turnCount++

		// 1. Task Start
		// (UI handled by stream renderer in caller)

		// Fresh isolated content replacement budget for this iteration so
		// delegated sub-agents never clobber the parent turn's budget.
		turnToolCtx := params.ToolUseContext
		turnToolCtx.ContentBudget = tools.NewDefaultContentBudget()

		// Yield stream start
		select {
		case out <- types.StreamRequestStart{}:
		case <-ctx.Done():
			out <- types.StreamError{Error: ctx.Err()}
			return Terminal{Reason: TerminalReasonUserInterrupt, Error: ctx.Err()}
		}

		// Token blocking check with auto-compaction
		if l.Config.AutoCompactEnabled && l.isAtBlockingLimit(state) {
			compaction, err := l.compactMessages(ctx, state, false)
			if err != nil {
				compactionErr := fmt.Errorf("automatic context compaction failed: %w", err)
				out <- types.StreamError{Error: compactionErr}
				return Terminal{Reason: TerminalReasonError, Error: compactionErr}
			}
			if err := emitCompaction(ctx, out, compaction); err != nil {
				out <- types.StreamError{Error: err}
				return Terminal{Reason: TerminalReasonUserInterrupt, Error: err}
			}
		}

		// Build the request only after compaction so this call uses the same
		// bounded context that callers persist from StreamContextCompacted.
		req := buildLLMRequest(params, state)

		// Call LLM
		var assistantMsg *types.Message
		var toolUses []types.ToolUseBlock
		var streamErr error
		llmEvents, err := l.Client.Stream(ctx, req)
		switch {
		case err == nil:
			assistantMsg, toolUses, streamErr = l.consumeStream(ctx, llmEvents, out)
		case isPromptTooLongError(err):
			// Providers usually refuse an oversized prompt with an HTTP error
			// before any stream starts. Recover from it the same way as a
			// refusal that arrives mid-stream.
			streamErr = &recoverableError{err: err, reason: "prompt_too_long"}
		default:
			out <- types.StreamError{Error: err}
			return Terminal{Reason: TerminalReasonError, Error: err}
		}

		// Handle recoverable errors with retry logic
		if streamErr != nil {
			if recErr, ok := streamErr.(*recoverableError); ok {
				recovered, newMsg, newToolUses, recoverErr := l.attemptRecovery(ctx, params, state, recErr, out)
				if recovered {
					assistantMsg = newMsg
					toolUses = newToolUses
					streamErr = nil
				} else if recoverErr != nil {
					// Recovery failed - now yield the original error
					streamErr = fmt.Errorf("recovery failed after %d attempts: %w (original: %v)",
						state.maxOutputTokensRecoveryCount, recoverErr, recErr.err)
				}
			}

			if streamErr != nil {
				out <- types.StreamError{Error: streamErr}
				return Terminal{Reason: TerminalReasonError, Error: streamErr}
			}
		}

		if assistantMsg == nil {
			return Terminal{Reason: TerminalReasonComplete}
		}

		state.add(*assistantMsg)

		if len(toolUses) == 0 {
			// No tools requested; turn is complete
			return Terminal{Reason: TerminalReasonComplete, Message: assistantMsg}
		}

		// Enforce total tool call budget per query
		maxToolCalls := l.Config.MaxToolCalls
		if params.MaxToolCalls > 0 {
			maxToolCalls = params.MaxToolCalls
		}
		if maxToolCalls <= 0 {
			maxToolCalls = DefaultLoopConfig().MaxToolCalls
		}
		if state.toolCallCount+len(toolUses) > maxToolCalls {
			// Suggest room for a larger task, bounded by the session ceiling.
			suggested := maxToolCalls * 5
			if suggested > maxToolLimit {
				suggested = maxToolLimit
			}
			msg := fmt.Sprintf("Tool call limit reached (%d tools). Runaway-loop protection stopped this turn. Type /limit %d to continue with a higher limit for this session.", maxToolCalls, suggested)
			select {
			case out <- types.StreamMessage{Message: types.Message{
				Role:    types.RoleSystem,
				Content: []types.ContentBlock{types.TextBlock{Text: msg}},
			}}:
			case <-ctx.Done():
				out <- types.StreamError{Error: ctx.Err()}
				return Terminal{Reason: TerminalReasonUserInterrupt, Error: ctx.Err()}
			}
			return Terminal{Reason: TerminalReasonBlockingLimit, Message: assistantMsg}
		}
		state.toolCallCount += len(toolUses)

		toolUses, err = l.prepareRepeatedTools(ctx, state, toolUses, params.ToolUseContext.Options.Tools, out)
		if err != nil {
			if ctx.Err() != nil {
				return Terminal{Reason: TerminalReasonUserInterrupt, Error: ctx.Err()}
			}
			return Terminal{Reason: TerminalReasonBlockingLimit, Message: assistantMsg, Error: err}
		}
		if len(toolUses) == 0 {
			continue // A rejected duplicate has a corrective result; let the model adapt.
		}

		// Execute tools
		if l.Config.StreamingToolExecution {
			executor := NewStreamingToolExecutor(params.ToolUseContext.Options.Tools, params.CanUseTool, turnToolCtx)
			for _, tu := range toolUses {
				executor.AddTool(tu, *assistantMsg)
			}

			// Consume events until all tools are done or cancellation stops
			// forwarding. Waiting on forwardDone before returning guarantees
			// this goroutine never touches out or state after Query closes out,
			// while a background drain keeps executor.dispatchEvents from
			// blocking if a slow tool ignores cancellation.
			stopForward := make(chan struct{})
			forwardDone := make(chan struct{})
			drainExecutor := func() {
				go func() {
					for range executor.Events() {
					}
				}()
			}
			go func() {
				defer close(forwardDone)
				for {
					select {
					case <-stopForward:
						drainExecutor()
						return
					case ev, ok := <-executor.Events():
						if !ok {
							return
						}
						select {
						case <-stopForward:
							drainExecutor()
							return
						default:
						}
						select {
						case out <- ev:
						case <-stopForward:
							drainExecutor()
							return
						case <-ctx.Done():
							drainExecutor()
							return
						}
						if sm, ok := ev.(types.StreamMessage); ok {
							state.add(sm.Message)
						}
					}
				}
			}()

			// Wait for completion
			_, execErr := executor.GetRemainingResults(ctx)
			executor.Close()
			if ctx.Err() != nil {
				close(stopForward)
				<-forwardDone
				out <- types.StreamError{Error: ctx.Err()}
				return Terminal{Reason: TerminalReasonUserInterrupt, Error: ctx.Err()}
			}
			<-forwardDone

			if execErr != nil {
				_ = execErr
			}
		} else {
			// Batch execution
			results, execErr := runToolsBatch(ctx, toolUses, *assistantMsg, turnToolCtx, params.CanUseTool)
			if execErr != nil {
				_ = execErr
			}
			for _, msg := range results {
				state.add(msg)
				select {
				case out <- types.StreamMessage{Message: msg}:
				case <-ctx.Done():
					out <- types.StreamError{Error: ctx.Err()}
					return Terminal{Reason: TerminalReasonUserInterrupt, Error: ctx.Err()}
				}
			}
			if ctx.Err() != nil {
				out <- types.StreamError{Error: ctx.Err()}
				return Terminal{Reason: TerminalReasonUserInterrupt, Error: ctx.Err()}
			}
		}
	}

	return Terminal{Reason: TerminalReasonMaxTurns}
}
