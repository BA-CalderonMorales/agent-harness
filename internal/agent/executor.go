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
	if !ok {
		e.tools = append(e.tools, trackedTool{
			id:                block.ID,
			block:             block,
			assistantMessage:  assistantMessage,
			status:            statusCompleted,
			isConcurrencySafe: true,
			results:           []types.Message{e.makeErrorMessage(block.ID, assistantMessage, fmt.Sprintf("Error: No such tool available: %s", block.Name))},
		})
		e.progressCond.Broadcast()
		return
	}

	safe := false
	if toolDef.Capabilities.IsConcurrencySafe != nil {
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
		e.events <- types.ProgressMessage{
			ToolUseID: t.block.ID,
			Type:      "progress",
			Data:      data,
			Timestamp: time.Now(),
		}
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
	e.events <- types.StreamMessage{Message: finalMsg}

	// Re-process queue now that a slot may have opened
	go e.processQueue()
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
	toolDef, ok := findTool(defs, block.Name)
	if !ok {
		return types.Message{}, fmt.Errorf("no such tool: %s", block.Name)
	}

	// Validate input
	if toolDef.ValidateInput != nil {
		vr := toolDef.ValidateInput(block.Input, ctx)
		if !vr.Valid {
			return types.Message{}, fmt.Errorf("validation failed: %s", vr.Message)
		}
	}

	// Permissions
	decision := toolDef.CheckPermissions(block.Input, ctx)
	if decision.Behavior == tools.Deny {
		return types.Message{}, fmt.Errorf("permission denied: %s", decision.Message)
	}
	if decision.Behavior == tools.Ask && canUseTool != nil {
		var err error
		decision, err = canUseTool(block.Name, block.Input, ctx)
		if err != nil {
			return types.Message{}, fmt.Errorf("permission check error: %w", err)
		}
		if decision.Behavior == tools.Deny {
			return types.Message{}, fmt.Errorf("permission denied: %s", decision.Message)
		}
	}

	input := block.Input
	if decision.UpdatedInput != nil {
		input = decision.UpdatedInput
	}

	// Backfill observable input
	if toolDef.BackfillObservableInput != nil {
		// Shallow clone
		cloned := make(map[string]any, len(input))
		for k, v := range input {
			cloned[k] = v
		}
		toolDef.BackfillObservableInput(cloned)
		input = cloned
	}

	// Call
	result, err := toolDef.Call(input, ctx, canUseTool, onProgress)
	if err != nil {
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
	return types.Message{
		Role:    types.RoleUser,
		Content: []types.ContentBlock{mapped},
	}, nil
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
