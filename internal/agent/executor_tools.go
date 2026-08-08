package agent

import (
	"context"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"time"
)

// AddTool enqueues a tool for execution.
func (e *StreamingToolExecutor) AddTool(block types.ToolUseBlock, assistantMessage types.Message) {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return
	}

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
	e.pendingTools.Add(1)
	e.tools = append(e.tools, tt)
	e.signalStateLocked()
	e.mu.Unlock()

	go e.processQueue()
}

// GetRemainingResults blocks until all tools complete and returns results in order.
func (e *StreamingToolExecutor) GetRemainingResults(ctx context.Context) ([]types.Message, error) {
	for {
		e.mu.Lock()
		if e.allToolsFinishedLocked() {
			var out []types.Message
			for i := range e.tools {
				out = append(out, e.tools[i].results...)
			}
			e.mu.Unlock()
			return out, nil
		}
		changed := e.stateChanged
		e.mu.Unlock()

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-changed:
		}
	}
}

func (e *StreamingToolExecutor) allToolsFinishedLocked() bool {
	for i := range e.tools {
		if e.tools[i].status != statusCompleted && e.tools[i].status != statusYielded {
			return false
		}
	}
	return true
}

func (e *StreamingToolExecutor) signalStateLocked() {
	close(e.stateChanged)
	e.stateChanged = make(chan struct{})
}

func (e *StreamingToolExecutor) processQueue() {
	e.mu.Lock()
	var ready []int
	for i := range e.tools {
		t := &e.tools[i]
		if t.status != statusQueued {
			continue
		}
		if e.canExecute(t.isConcurrencySafe) {
			t.status = statusExecuting
			ready = append(ready, i)
		} else if !t.isConcurrencySafe {
			// Non-concurrent tool blocked; stop here to preserve order
			break
		}
	}
	if len(ready) > 0 {
		e.signalStateLocked()
	}
	e.mu.Unlock()

	for _, index := range ready {
		go e.executeTool(index)
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

func (e *StreamingToolExecutor) executeTool(index int) {
	defer e.pendingTools.Done()

	e.mu.Lock()
	t := e.tools[index]
	e.mu.Unlock()

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
	tracked := &e.tools[index]
	var finalMsg types.Message
	if e.discarded {
		finalMsg = e.makeErrorMessage(t.block.ID, t.assistantMessage, "Tool execution discarded due to streaming fallback")
	} else if err != nil {
		finalMsg = e.makeErrorMessage(t.block.ID, t.assistantMessage, err.Error())
		if isBashTool(t.block.Name) {
			e.hasErrored = true
			e.siblingCancel()
		}
	} else {
		finalMsg = result
	}
	tracked.results = []types.Message{finalMsg}
	tracked.status = statusCompleted
	finalEvents := e.collectReadyResultEventsLocked()
	e.signalStateLocked()
	e.enqueueEvents(finalEvents)
	e.mu.Unlock()

	// Re-process queue now that a slot may have opened
	go e.processQueue()
}

func (e *StreamingToolExecutor) collectReadyResultEventsLocked() []types.StreamEvent {
	var events []types.StreamEvent
	for e.nextResultEvent < len(e.tools) {
		tracked := &e.tools[e.nextResultEvent]
		if tracked.status != statusCompleted && tracked.status != statusYielded {
			break
		}
		for _, result := range tracked.results {
			events = append(events, types.StreamMessage{Message: result})
		}
		e.nextResultEvent++
	}
	return events
}

// sendEvent appends to the dispatcher-owned event queue. Workers never send
// to or close the public channel directly.
func (e *StreamingToolExecutor) sendEvent(ev types.StreamEvent) {
	e.enqueueEvents([]types.StreamEvent{ev})
}

func (e *StreamingToolExecutor) enqueueEvents(events []types.StreamEvent) {
	if len(events) == 0 {
		return
	}
	e.eventMu.Lock()
	if !e.eventQueueClosed {
		e.eventQueue = append(e.eventQueue, events...)
		e.eventCond.Signal()
	}
	e.eventMu.Unlock()
}

func (e *StreamingToolExecutor) makeErrorMessage(toolUseID string, assistantMsg types.Message, text string) types.Message {
	return types.Message{
		UUID:      assistantMsg.UUID + "_err_" + toolUseID,
		Role:      types.RoleUser,
		Content:   []types.ContentBlock{types.ToolResultBlock{ToolUseID: toolUseID, Content: text, IsError: true}},
		Timestamp: assistantMsg.Timestamp,
	}
}
