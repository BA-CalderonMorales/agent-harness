package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// TrackedTool represents a tool in the execution queue.
type trackedTool struct {
	id                string
	block             types.ToolUseBlock
	assistantMessage  types.Message
	status            toolStatus
	isConcurrencySafe bool
	results           []types.Message
	contextModifiers  []func(ctx tools.Context) tools.Context
	promise           chan struct{}
}

type toolStatus string

const (
	statusQueued    toolStatus = "queued"
	statusExecuting toolStatus = "executing"
	statusCompleted toolStatus = "completed"
	statusYielded   toolStatus = "yielded"
)

// StreamingToolExecutor manages concurrent tool execution with ordering guarantees.
type StreamingToolExecutor struct {
	tools           []trackedTool
	canUseTool      tools.CanUseToolFn
	toolDefinitions []tools.Tool
	toolUseContext  tools.Context
	hasErrored      bool
	siblingCtx      context.Context
	siblingCancel   context.CancelFunc
	discarded       bool
	mu              sync.Mutex
	progressCond    *sync.Cond
	events          chan types.StreamEvent
	closed          bool
}

// NewStreamingToolExecutor creates a new executor.
func NewStreamingToolExecutor(toolDefs []tools.Tool, canUseTool tools.CanUseToolFn, ctx tools.Context) *StreamingToolExecutor {
	siblingCtx, cancel := context.WithCancel(ctx.AbortController)
	e := &StreamingToolExecutor{
		tools:           make([]trackedTool, 0),
		canUseTool:      canUseTool,
		toolDefinitions: toolDefs,
		toolUseContext:  ctx,
		siblingCtx:      siblingCtx,
		siblingCancel:   cancel,
		events:          make(chan types.StreamEvent, 16),
	}
	e.progressCond = sync.NewCond(&e.mu)
	return e
}

// Events returns the stream of tool events (progress and results).
func (e *StreamingToolExecutor) Events() <-chan types.StreamEvent {
	return e.events
}

// Close closes the events channel. Should be called when all tools are done.
// Safe to call multiple times.
func (e *StreamingToolExecutor) Close() {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return
	}
	e.closed = true
	e.mu.Unlock()
	close(e.events)
}

// Discard abandons all pending and in-progress tools.
func (e *StreamingToolExecutor) Discard() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.discarded = true
	e.siblingCancel()
}

// DiscardRespectingInterrupt abandons tools based on their interrupt behavior.
// Tools with "block" behavior continue running; tools with "cancel" behavior are stopped.
func (e *StreamingToolExecutor) DiscardRespectingInterrupt(toolDefs []tools.Tool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Find which tools are running and check their interrupt behavior
	for i := range e.tools {
		if e.tools[i].status != statusExecuting {
			continue
		}

		toolDef, ok := findTool(toolDefs, e.tools[i].block.Name)
		if !ok {
			continue
		}

		behavior := "block" // default
		if toolDef.Capabilities.InterruptBehavior != nil {
			behavior = toolDef.Capabilities.InterruptBehavior()
		}

		// Only cancel tools that allow cancellation
		if behavior == "cancel" {
			// Tool will be cancelled via context
			e.siblingCancel()
		}
		// Tools with "block" behavior continue running
	}
}

// AddTool enqueues a tool for execution.
func (e *StreamingToolExecutor) AddTool(block types.ToolUseBlock, assistantMessage types.Message) {
	e.mu.Lock()
	defer e.mu.Unlock()

	toolDef, ok := findTool(e.toolDefinitions, block.Name)
	safe := false
	if ok && toolDef.Capabilities.IsConcurrencySafe != nil {
		safe = toolDef.Capabilities.IsConcurrencySafe(block.Input)
	}

	tt := trackedTool{
		id:                block.ID,
		block:             block,
		assistantMessage:  assistantMessage,
		status:            statusQueued,
		isConcurrencySafe: safe,
		promise:           make(chan struct{}),
	}
	e.tools = append(e.tools, tt)
	go e.processQueue()
}

// GetRemainingResults blocks until all tools complete and returns results in order.
func (e *StreamingToolExecutor) GetRemainingResults(ctx context.Context) ([]types.Message, error) {
	e.mu.Lock()
	for {
		done := true
		for i := range e.tools {
			if e.tools[i].status != statusCompleted && e.tools[i].status != statusYielded {
				done = false
				break
			}
		}
		if done {
			break
		}
		e.progressCond.Wait()
	}
	e.mu.Unlock()

	var out []types.Message
	for i := range e.tools {
		out = append(out, e.tools[i].results...)
	}
	return out, nil
}

func (e *StreamingToolExecutor) processQueue() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i := range e.tools {
		t := &e.tools[i]
		if t.status != statusQueued {
			continue
		}
		if e.canExecute(t.isConcurrencySafe) {
			t.status = statusExecuting
			go e.executeTool(t)
		} else if !t.isConcurrencySafe {
			// Non-concurrent tool blocked; stop here to preserve order
			break
		}
	}
}

func (e *StreamingToolExecutor) canExecute(isSafe bool) bool {
	executing := 0
	for i := range e.tools {
		if e.tools[i].status == statusExecuting {
			executing++
		}
	}
	if executing == 0 {
		return true
	}
	if !isSafe {
		return false
	}
	// Safe tools can run alongside other safe tools
	for i := range e.tools {
		if e.tools[i].status == statusExecuting && !e.tools[i].isConcurrencySafe {
			return false
		}
	}
	return true
}

func (e *StreamingToolExecutor) executeTool(t *trackedTool) {
	ctx := e.toolUseContext
	// Apply context modifiers from previous non-concurrent tools
	for _, mod := range t.contextModifiers {
		ctx = mod(ctx)
	}

	// Determine abort context: use sibling context for Bash tools
	toolCtx := ctx.AbortController
	if isBashTool(t.block.Name) {
		toolCtx = e.siblingCtx
	}
	ctx.AbortController = toolCtx

	// Progress handler
	onProgress := func(data any) {
		e.sendEvent(types.ProgressMessage{
			ToolUseID: t.block.ID,
			Type:      "progress",
			Data:      data,
			Timestamp: time.Now(),
		})
	}

	result, err := runSingleTool(ctx, t.block, t.assistantMessage, e.toolDefinitions, e.canUseTool, onProgress)

	e.mu.Lock()

	if e.discarded {
		t.status = statusCompleted
		t.results = []types.Message{e.makeErrorMessage(t.block.ID, t.assistantMessage, "Tool execution discarded due to streaming fallback")}
		e.mu.Unlock()
		e.progressCond.Broadcast()
		go e.processQueue()
		return
	}

	var finalMsg types.Message
	if err != nil {
		finalMsg = e.makeErrorMessage(t.block.ID, t.assistantMessage, err.Error())
		t.results = []types.Message{finalMsg}
		if isBashTool(t.block.Name) {
			e.hasErrored = true
			e.siblingCancel()
		}
	} else {
		finalMsg = result
		t.results = []types.Message{finalMsg}
	}
	t.status = statusCompleted

	e.mu.Unlock()
	e.progressCond.Broadcast()

	// Stream the final result event
	e.sendEvent(types.StreamMessage{Message: finalMsg})

	// Re-process queue now that a slot may have opened
	go e.processQueue()
}

// sendEvent delivers an event to the events channel, swallowing the panic
// that occurs if the channel has already been closed. This prevents races
// between tool goroutines finishing and the consumer calling Close().
func (e *StreamingToolExecutor) sendEvent(ev types.StreamEvent) {
	defer func() { recover() }()
	e.events <- ev
}

func (e *StreamingToolExecutor) makeErrorMessage(toolUseID string, assistantMsg types.Message, text string) types.Message {
	return types.Message{
		UUID:      assistantMsg.UUID + "_err_" + toolUseID,
		Role:      types.RoleUser,
		Content:   []types.ContentBlock{types.ToolResultBlock{ToolUseID: toolUseID, Content: text, IsError: true}},
		Timestamp: assistantMsg.Timestamp,
	}
}

func findTool(defs []tools.Tool, name string) (tools.Tool, bool) {
	for _, t := range defs {
		if t.Name == name {
			return t, true
		}
		for _, a := range t.Aliases {
			if a == name {
				return t, true
			}
		}
	}
	return tools.Tool{}, false
}

func isBashTool(name string) bool {
	return name == "bash" || name == "BashTool"
}

// runSingleTool executes one tool call.
func runSingleTool(ctx tools.Context, block types.ToolUseBlock, assistantMsg types.Message, defs []tools.Tool, canUseTool tools.CanUseToolFn, onProgress tools.OnProgress) (types.Message, error) {
	toolDef, toolFound := findTool(defs, block.Name)

	// Work on a private input map so canonicalization hooks cannot mutate the
	// assistant message or another policy layer's view of the proposal.
	input := make(map[string]any, len(block.Input))
	for key, value := range block.Input {
		input[key] = value
	}

	var validationErr error
	if toolFound && toolDef.ValidateInput != nil {
		vr := toolDef.ValidateInput(block.Input, ctx)
		if !vr.Valid {
			validationErr = fmt.Errorf("validation failed: %s", vr.Message)
		}
	}

	// Canonicalize before either policy layer sees the proposal.
	if toolFound && toolDef.BackfillObservableInput != nil {
		toolDef.BackfillObservableInput(input)
	}

	globalDecision := tools.PermissionDecision{
		Behavior:     tools.Allow,
		UpdatedInput: input,
	}
	if canUseTool != nil {
		var err error
		globalDecision, err = canUseTool(block.Name, input, ctx)
		if err != nil {
			return types.Message{}, fmt.Errorf("permission check error: %w", err)
		}
	} else if ctx.RequireCanUseTool {
		return types.Message{}, fmt.Errorf("permission denied: global policy is unavailable")
	}

	if globalDecision.UpdatedInput != nil {
		input = globalDecision.UpdatedInput
	}

	auditEvent := func(event string, behavior tools.DecisionBehavior, message string, durationMillis int64, eventErr error) error {
		if globalDecision.Audit == nil {
			return nil
		}
		return globalDecision.Audit(tools.ToolAuditEvent{
			Event:          event,
			ToolCallID:     block.ID,
			ToolName:       block.Name,
			Input:          input,
			Behavior:       behavior,
			Message:        message,
			DurationMillis: durationMillis,
			Err:            eventErr,
		})
	}

	if err := auditEvent("proposal", globalDecision.Behavior, globalDecision.Message, 0, nil); err != nil {
		return types.Message{}, fmt.Errorf("permission denied: durable proposal audit failed: %w", err)
	}

	if !toolFound {
		decisionErr := fmt.Errorf("Error: No such tool available: %s", block.Name)
		if err := auditEvent("decision", tools.Deny, decisionErr.Error(), 0, decisionErr); err != nil {
			return types.Message{}, fmt.Errorf("%v (decision audit failed: %w)", decisionErr, err)
		}
		return types.Message{}, decisionErr
	}

	if validationErr != nil {
		if err := auditEvent("decision", tools.Deny, validationErr.Error(), 0, validationErr); err != nil {
			return types.Message{}, fmt.Errorf("%v (decision audit failed: %w)", validationErr, err)
		}
		return types.Message{}, validationErr
	}

	localDecision := tools.PermissionDecision{
		Behavior:     tools.Allow,
		UpdatedInput: input,
	}
	if toolDef.CheckPermissions != nil {
		localDecision = toolDef.CheckPermissions(input, ctx)
	}
	decision := mergePermissionDecisions(globalDecision, localDecision)
	if decision.UpdatedInput != nil {
		input = decision.UpdatedInput
	}

	if decision.Behavior == tools.Deny {
		decisionErr := fmt.Errorf("permission denied: %s", decision.Message)
		if err := auditEvent("decision", tools.Deny, decision.Message, 0, decisionErr); err != nil {
			return types.Message{}, fmt.Errorf("%v (decision audit failed: %w)", decisionErr, err)
		}
		return types.Message{}, decisionErr
	}

	if decision.Behavior == tools.Ask {
		if globalDecision.Checkpoint == nil {
			decisionErr := fmt.Errorf("permission denied: approval checkpoint is unavailable")
			if err := auditEvent("decision", tools.Deny, decisionErr.Error(), 0, decisionErr); err != nil {
				return types.Message{}, fmt.Errorf("%v (decision audit failed: %w)", decisionErr, err)
			}
			return types.Message{}, decisionErr
		}

		checkpointDecision, err := globalDecision.Checkpoint()
		if err != nil {
			decisionErr := fmt.Errorf("approval checkpoint failed: %w", err)
			if auditErr := auditEvent("decision", tools.Deny, decisionErr.Error(), 0, decisionErr); auditErr != nil {
				return types.Message{}, fmt.Errorf("%v (decision audit failed: %w)", decisionErr, auditErr)
			}
			return types.Message{}, decisionErr
		}
		if checkpointDecision.UpdatedInput != nil {
			input = checkpointDecision.UpdatedInput
		}
		if checkpointDecision.Behavior != tools.Allow {
			decisionErr := fmt.Errorf("permission denied: %s", checkpointDecision.Message)
			if auditErr := auditEvent("decision", tools.Deny, checkpointDecision.Message, 0, decisionErr); auditErr != nil {
				return types.Message{}, fmt.Errorf("%v (decision audit failed: %w)", decisionErr, auditErr)
			}
			return types.Message{}, decisionErr
		}
		decision.Behavior = tools.Allow
		decision.Message = checkpointDecision.Message
	}

	if err := auditEvent("decision", tools.Allow, decision.Message, 0, nil); err != nil {
		return types.Message{}, fmt.Errorf("permission denied: durable decision audit failed: %w", err)
	}

	started := time.Now()
	if err := auditEvent("start", tools.Allow, "", 0, nil); err != nil {
		return types.Message{}, fmt.Errorf("tool execution blocked: durable start audit failed: %w", err)
	}

	result, err := toolDef.Call(input, ctx, canUseTool, onProgress)
	if err != nil {
		durationMillis := time.Since(started).Milliseconds()
		if auditErr := auditEvent("failure", tools.Allow, err.Error(), durationMillis, err); auditErr != nil {
			return types.Message{}, fmt.Errorf("tool failed: %v (outcome audit failed: %w)", err, auditErr)
		}
		return types.Message{}, err
	}

	// Apply content replacement budget
	budget := tools.GetCurrentBudget()
	resultStr := fmt.Sprintf("%v", result.Data)

	if !budget.CanUseResult(block.Name, len(resultStr), int64(toolDef.MaxResultSizeChars)) {
		// Truncate to fit budget
		truncated, note := budget.GetTruncatedResult(block.Name, resultStr, int64(toolDef.MaxResultSizeChars))
		result.Data = truncated
		_ = note // note is included in truncated result
	} else {
		// Record usage
		_ = budget.RecordUsage(block.Name, len(resultStr), int64(toolDef.MaxResultSizeChars))
	}

	mapped := toolDef.MapResult(result.Data, block.ID)
	if err := auditEvent("success", tools.Allow, "", time.Since(started).Milliseconds(), nil); err != nil {
		return types.Message{}, fmt.Errorf("tool succeeded but outcome audit failed: %w", err)
	}
	return types.Message{
		Role:    types.RoleUser,
		Content: []types.ContentBlock{mapped},
	}, nil
}

func mergePermissionDecisions(global, local tools.PermissionDecision) tools.PermissionDecision {
	merged := global
	if local.UpdatedInput != nil {
		merged.UpdatedInput = local.UpdatedInput
	}

	if global.Behavior == tools.Deny {
		return merged
	}
	if global.Behavior != tools.Allow && global.Behavior != tools.Ask {
		merged.Behavior = tools.Deny
		merged.Message = "unsupported global permission decision"
		return merged
	}

	switch local.Behavior {
	case tools.Deny:
		merged.Behavior = tools.Deny
		merged.Message = local.Message
	case tools.Ask:
		merged.Behavior = tools.Ask
		if local.Message != "" {
			merged.Message = local.Message
		}
	case tools.Allow:
		if global.Behavior == tools.Ask {
			merged.Behavior = tools.Ask
		} else {
			merged.Behavior = tools.Allow
		}
	default:
		merged.Behavior = tools.Deny
		merged.Message = "unsupported tool-specific permission decision"
	}
	return merged
}

// runToolsBatch executes a batch of tools with partitioning.
func runToolsBatch(ctx context.Context, blocks []types.ToolUseBlock, assistantMsg types.Message, toolCtx tools.Context, canUseTool tools.CanUseToolFn) ([]types.Message, error) {
	var out []types.Message
	for _, block := range blocks {
		msg, err := runSingleTool(toolCtx, block, assistantMsg, toolCtx.Options.Tools, canUseTool, nil)
		if err != nil {
			msg = types.Message{
				Role:    types.RoleUser,
				Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: block.ID, Content: err.Error(), IsError: true}},
			}
		}
		out = append(out, msg)
	}
	return out, nil
}
