// Package tools provides the modular tool architecture.
//
// Tools are decomposed into focused interfaces that can be implemented
// independently by "buckets" - domain-specific tool implementations.
//
// Core principle: Each bucket implements ToolBase but only handles
// what it knows. The ToolOrchestrator coordinates without digging
// into bucket internals.
package tools

import (
	"context"
	"fmt"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// ToolBase is the fundamental interface all tool buckets must implement.
// It defines the contract for participating in tool execution without
// prescribing implementation details.
type ToolBase interface {
	// Name returns the bucket identifier (e.g., "filesystem", "shell", "search")
	Name() string

	// CanHandle determines if this bucket should handle the given tool call.
	// This allows the orchestrator to route tool calls to appropriate buckets.
	CanHandle(toolName string, input map[string]any) bool

	// Execute runs the tool call and returns results.
	// The bucket handles all internals: validation, permissions, execution.
	Execute(ctx ToolExecutionContext) ToolResult

	// Capabilities describes what this bucket can do.
	Capabilities() ToolBucketCapabilities

	// GetTools returns the tool definitions this bucket provides.
	GetTools() []Tool
}

// ToolExecutionContext carries all context needed for a single tool execution.
// Buckets receive this instead of digging into global state.
type ToolExecutionContext struct {
	Context    context.Context
	ToolName   string
	Input      map[string]any
	ToolUseID  string
	Messages   []types.Message
	CanUseTool CanUseToolFn
	OnProgress OnProgress
	ToolCtx    Context // The tools.Context with Options, AbortController, etc.
}

// ToolBucketCapabilities describes static capabilities of a tool bucket.
type ToolBucketCapabilities struct {
	IsConcurrencySafe bool     // Can run alongside other safe buckets
	IsReadOnly        bool     // Doesn't modify state
	IsDestructive     bool     // Can destroy/modify state
	ToolNames         []string // Tools this bucket handles
	Category          string   // "filesystem", "shell", "search", etc.
}

// ToolOrchestrator coordinates multiple ToolBase implementations.
// It routes tool calls to appropriate buckets without knowing their internals.
type ToolOrchestrator struct {
	buckets  []ToolBase
	registry *ToolRegistry
}

// NewToolOrchestrator creates a new orchestrator with the given buckets.
func NewToolOrchestrator(buckets ...ToolBase) *ToolOrchestrator {
	registry := NewRegistry()
	for _, b := range buckets {
		for _, t := range b.GetTools() {
			registry.RegisterBuiltIn(t)
		}
	}
	return &ToolOrchestrator{
		buckets:  buckets,
		registry: registry,
	}
}

// RegisterBucket adds a bucket to the orchestrator.
func (o *ToolOrchestrator) RegisterBucket(bucket ToolBase) {
	o.buckets = append(o.buckets, bucket)
	for _, t := range bucket.GetTools() {
		o.registry.RegisterBuiltIn(t)
	}
}

// FindBucket locates the appropriate bucket for a tool.
func (o *ToolOrchestrator) FindBucket(toolName string, input map[string]any) (ToolBase, bool) {
	for _, bucket := range o.buckets {
		if bucket.CanHandle(toolName, input) {
			return bucket, true
		}
	}
	return nil, false
}

// GetAllTools returns all tools from all buckets.
func (o *ToolOrchestrator) GetAllTools() []Tool {
	return o.registry.AllTools()
}

// GetRegistry returns the underlying registry.
func (o *ToolOrchestrator) GetRegistry() *ToolRegistry {
	return o.registry
}

// ExecuteTool routes and executes a single tool call.
func (o *ToolOrchestrator) ExecuteTool(ctx ToolExecutionContext) ToolResult {
	bucket, found := o.FindBucket(ctx.ToolName, ctx.Input)
	if !found {
		return ToolResult{
			Data: fmt.Errorf("no bucket found for tool: %s", ctx.ToolName),
		}
	}
	return bucket.Execute(ctx)
}

// ToolError represents an error from tool execution.
type ToolError struct {
	Code    string
	Message string
	Cause   error
}

func (e *ToolError) Error() string {
	if e.Cause != nil {
		return e.Code + ": " + e.Message + " (caused by: " + e.Cause.Error() + ")"
	}
	return e.Code + ": " + e.Message
}

// NewToolError creates a new tool error.
func NewToolError(code, message string) *ToolError {
	return &ToolError{Code: code, Message: message}
}

// WrapToolError wraps an existing error.
func WrapToolError(code string, cause error) *ToolError {
	return &ToolError{Code: code, Message: cause.Error(), Cause: cause}
}
