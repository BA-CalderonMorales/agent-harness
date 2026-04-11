package executor

import (
	"fmt"
	"sync"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent/core"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// ExecutorBucket handles tool execution.
// It implements AgentBase for the core execution logic.
type ExecutorBucket struct{}

// Executor creates a new executor bucket.
func Executor() *ExecutorBucket {
	return &ExecutorBucket{}
}

// Name returns the bucket identifier.
func (e *ExecutorBucket) Name() string {
	return "executor"
}

// CanHandle determines if this bucket handles the operation.
func (e *ExecutorBucket) CanHandle(operation string, params map[string]any) bool {
	switch operation {
	case "execute_tools", "execute_single", "run_batch":
		return true
	}
	return false
}

// Capabilities describes what this bucket can do.
func (e *ExecutorBucket) Capabilities() agent.AgentBucketCapabilities {
	return agent.AgentBucketCapabilities{
		IsConcurrencySafe: true,
		IsStateful:        false,
		Operations:        []string{"execute_tools", "execute_single", "run_batch"},
		Category:          "executor",
	}
}

// Execute runs the executor operation.
func (e *ExecutorBucket) Execute(ctx agent.AgentExecutionContext) agent.AgentResult {
	switch ctx.Operation {
	case "execute_tools":
		return e.handleExecuteTools(ctx)
	case "execute_single":
		return e.handleExecuteSingle(ctx)
	case "run_batch":
		return e.handleRunBatch(ctx)
	default:
		return agent.AgentResult{
			Success: false,
			Error:   agent.NewAgentError("unknown_operation", "executor doesn't handle: "+ctx.Operation),
		}
	}
}

// handleExecuteTools executes multiple tools with concurrency safety.
func (e *ExecutorBucket) handleExecuteTools(ctx agent.AgentExecutionContext) agent.AgentResult {
	blocks, ok := ctx.Params["tool_use_blocks"].([]types.ToolUseBlock)
	if !ok || len(blocks) == 0 {
		return agent.AgentResult{
			Success: true,
			Data:    "no tools to execute",
		}
	}

	// Partition tools by concurrency safety
	safeBlocks, unsafeBlocks := e.partitionTools(blocks, ctx.Tools)

	var results []types.Message

	// Execute safe tools concurrently
	if len(safeBlocks) > 0 {
		safeResults := e.executeConcurrently(ctx, safeBlocks)
		results = append(results, safeResults...)
	}

	// Execute unsafe tools sequentially
	for _, block := range unsafeBlocks {
		result := e.executeSingle(ctx, block)
		results = append(results, result)
	}

	return agent.AgentResult{
		Success:  true,
		Data:     results,
		Messages: results,
	}
}

// handleExecuteSingle executes a single tool.
func (e *ExecutorBucket) handleExecuteSingle(ctx agent.AgentExecutionContext) agent.AgentResult {
	block, ok := ctx.Params["tool_use_block"].(types.ToolUseBlock)
	if !ok {
		return agent.AgentResult{
			Success: false,
			Error:   agent.NewAgentError("invalid_input", "tool_use_block required"),
		}
	}

	result := e.executeSingle(ctx, block)
	return agent.AgentResult{
		Success:  true,
		Data:     result,
		Messages: []types.Message{result},
	}
}

// handleRunBatch executes a batch of tools.
func (e *ExecutorBucket) handleRunBatch(ctx agent.AgentExecutionContext) agent.AgentResult {
	blocks, ok := ctx.Params["tool_use_blocks"].([]types.ToolUseBlock)
	if !ok {
		return agent.AgentResult{
			Success: false,
			Error:   agent.NewAgentError("invalid_input", "tool_use_blocks required"),
		}
	}

	var results []types.Message
	for _, block := range blocks {
		result := e.executeSingle(ctx, block)
		results = append(results, result)
	}

	return agent.AgentResult{
		Success:  true,
		Data:     results,
		Messages: results,
	}
}

// partitionTools separates tools by concurrency safety.
func (e *ExecutorBucket) partitionTools(blocks []types.ToolUseBlock, toolDefs []tools.Tool) (safe []types.ToolUseBlock, unsafe []types.ToolUseBlock) {
	for _, block := range blocks {
		toolDef, found := e.findTool(toolDefs, block.Name)
		if !found {
			// Unknown tool - treat as unsafe
			unsafe = append(unsafe, block)
			continue
		}

		if toolDef.Capabilities.IsConcurrencySafe != nil && toolDef.Capabilities.IsConcurrencySafe(block.Input) {
			safe = append(safe, block)
		} else {
			unsafe = append(unsafe, block)
		}
	}
	return safe, unsafe
}

// executeConcurrently runs safe tools concurrently.
func (e *ExecutorBucket) executeConcurrently(ctx agent.AgentExecutionContext, blocks []types.ToolUseBlock) []types.Message {
	var wg sync.WaitGroup
	results := make([]types.Message, len(blocks))

	for i, block := range blocks {
		wg.Add(1)
		go func(idx int, b types.ToolUseBlock) {
			defer wg.Done()
			results[idx] = e.executeSingle(ctx, b)
		}(i, block)
	}

	wg.Wait()
	return results
}

// executeSingle runs a single tool.
func (e *ExecutorBucket) executeSingle(ctx agent.AgentExecutionContext, block types.ToolUseBlock) types.Message {
	toolDef, found := e.findTool(ctx.Tools, block.Name)
	if !found {
		return types.Message{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: block.ID, Content: fmt.Sprintf("Error: No such tool: %s", block.Name), IsError: true}},
		}
	}

	// Validate input
	if toolDef.ValidateInput != nil {
		vr := toolDef.ValidateInput(block.Input, ctx.ToolContext)
		if !vr.Valid {
			return types.Message{
				Role:    types.RoleUser,
				Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: block.ID, Content: fmt.Sprintf("Validation failed: %s", vr.Message), IsError: true}},
			}
		}
	}

	// Check permissions
	decision := toolDef.CheckPermissions(block.Input, ctx.ToolContext)
	if decision.Behavior == tools.Deny {
		return types.Message{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: block.ID, Content: fmt.Sprintf("Permission denied: %s", decision.Message), IsError: true}},
		}
	}
	if decision.Behavior == tools.Ask && ctx.CanUseTool != nil {
		var err error
		decision, err = ctx.CanUseTool(block.Name, block.Input, ctx.ToolContext)
		if err != nil || decision.Behavior == tools.Deny {
			return types.Message{
				Role:    types.RoleUser,
				Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: block.ID, Content: "Permission denied", IsError: true}},
			}
		}
	}

	input := block.Input
	if decision.UpdatedInput != nil {
		input = decision.UpdatedInput
	}

	// Progress handler
	onProgress := func(data any) {
		// Progress updates can be sent through a channel if needed
		_ = data
	}

	// Execute
	result, err := toolDef.Call(input, ctx.ToolContext, ctx.CanUseTool, onProgress)
	if err != nil {
		return types.Message{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: block.ID, Content: err.Error(), IsError: true}},
		}
	}

	mapped := toolDef.MapResult(result.Data, block.ID)
	return types.Message{
		Role:      types.RoleUser,
		Content:   []types.ContentBlock{mapped},
		Timestamp: time.Now(),
	}
}

// findTool locates a tool definition by name.
func (e *ExecutorBucket) findTool(defs []tools.Tool, name string) (tools.Tool, bool) {
	for _, t := range defs {
		if t.Name == name {
			return t, true
		}
		for _, alias := range t.Aliases {
			if alias == name {
				return t, true
			}
		}
	}
	return tools.Tool{}, false
}

// Ensure ExecutorBucket implements AgentBase
var _ agent.AgentBase = (*ExecutorBucket)(nil)
