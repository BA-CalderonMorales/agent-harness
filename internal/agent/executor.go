package agent

import (
	"context"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"sync"
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
	stateChanged    chan struct{}
	pendingTools    sync.WaitGroup
	closeOnce       sync.Once
	closed          bool
	nextResultEvent int

	eventMu          sync.Mutex
	eventCond        *sync.Cond
	eventQueue       []types.StreamEvent
	eventQueueClosed bool
	events           chan types.StreamEvent
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
		stateChanged:    make(chan struct{}),
		events:          make(chan types.StreamEvent, 16),
	}
	e.eventCond = sync.NewCond(&e.eventMu)
	go e.dispatchEvents()
	return e
}

// Events returns the stream of tool events (progress and results).
func (e *StreamingToolExecutor) Events() <-chan types.StreamEvent {
	return e.events
}

// Close seals admissions, waits for every accepted tool to publish its final
// event, and then tells the sole event dispatcher to close the public stream.
func (e *StreamingToolExecutor) Close() {
	e.closeOnce.Do(func() {
		e.mu.Lock()
		e.closed = true
		e.mu.Unlock()

		e.pendingTools.Wait()
		e.siblingCancel()

		e.eventMu.Lock()
		e.eventQueueClosed = true
		e.eventCond.Broadcast()
		e.eventMu.Unlock()
	})
}

func (e *StreamingToolExecutor) dispatchEvents() {
	defer close(e.events)

	for {
		e.eventMu.Lock()
		for len(e.eventQueue) == 0 && !e.eventQueueClosed {
			e.eventCond.Wait()
		}
		if len(e.eventQueue) == 0 && e.eventQueueClosed {
			e.eventMu.Unlock()
			return
		}
		event := e.eventQueue[0]
		e.eventQueue[0] = nil
		e.eventQueue = e.eventQueue[1:]
		if len(e.eventQueue) == 0 {
			e.eventQueue = nil
		}
		e.eventMu.Unlock()
		e.events <- event
	}
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
