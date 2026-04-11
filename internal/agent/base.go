// Package agent provides the modular agent architecture.
//
// Agents are decomposed into focused interfaces that can be implemented
// independently by "buckets" - domain-specific agent components.
//
// Core principle: Each bucket implements AgentBase but only handles
// what it knows. The AgentOrchestrator coordinates without digging
// into bucket internals.
package agent

import (
	"context"

	"github.com/BA-CalderonMorales/agent-harness/internal/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// AgentBase is the fundamental interface all agent buckets must implement.
// It defines the contract for participating in agent execution without
// prescribing implementation details.
type AgentBase interface {
	// Name returns the bucket identifier (e.g., "executor", "conversation", "cost")
	Name() string

	// CanHandle determines if this bucket should handle the given operation.
	CanHandle(operation string, params map[string]any) bool

	// Execute runs the agent operation and returns results.
	Execute(ctx AgentExecutionContext) AgentResult

	// Capabilities describes what this bucket can do.
	Capabilities() AgentBucketCapabilities
}

// AgentExecutionContext carries all context needed for agent execution.
// Buckets receive this instead of digging into global state.
type AgentExecutionContext struct {
	Context     context.Context
	Operation   string
	Params      map[string]any
	Messages    []types.Message
	Tools       []tools.Tool
	LLMClient   llm.Client
	CanUseTool  tools.CanUseToolFn
	ToolContext tools.Context
}

// AgentResult is the standardized output from agent bucket execution.
type AgentResult struct {
	Success      bool
	Data         any
	Messages     []types.Message
	Error        error
	Cost         CostInfo
	ShouldHalt   bool
	Retryable    bool
}

// CostInfo tracks token and cost information.
type CostInfo struct {
	InputTokens  int
	OutputTokens int
	TotalCost    float64
	Model        string
}

// AgentBucketCapabilities describes static capabilities of an agent bucket.
type AgentBucketCapabilities struct {
	IsConcurrencySafe bool     // Can run alongside other safe buckets
	IsStateful        bool     // Maintains state between calls
	Operations        []string // Operations this bucket handles
	Category          string   // "executor", "conversation", "cost", etc.
}

// AgentOrchestrator coordinates multiple AgentBase implementations.
// It routes operations to appropriate buckets without knowing their internals.
type AgentOrchestrator struct {
	buckets []AgentBase
}

// NewAgentOrchestrator creates a new orchestrator with the given buckets.
func NewAgentOrchestrator(buckets ...AgentBase) *AgentOrchestrator {
	return &AgentOrchestrator{
		buckets: buckets,
	}
}

// RegisterBucket adds a bucket to the orchestrator.
func (o *AgentOrchestrator) RegisterBucket(bucket AgentBase) {
	o.buckets = append(o.buckets, bucket)
}

// FindBucket locates the appropriate bucket for an operation.
func (o *AgentOrchestrator) FindBucket(operation string, params map[string]any) (AgentBase, bool) {
	for _, bucket := range o.buckets {
		if bucket.CanHandle(operation, params) {
			return bucket, true
		}
	}
	return nil, false
}

// Execute routes and executes an operation.
func (o *AgentOrchestrator) Execute(ctx AgentExecutionContext) AgentResult {
	bucket, found := o.FindBucket(ctx.Operation, ctx.Params)
	if !found {
		return AgentResult{
			Success: false,
			Error:   ErrNoBucket,
		}
	}
	return bucket.Execute(ctx)
}

// ErrNoBucket is returned when no bucket can handle an operation.
var ErrNoBucket = &AgentError{Code: "no_bucket", Message: "no bucket found for operation"}

// AgentError represents an error from agent execution.
type AgentError struct {
	Code    string
	Message string
	Cause   error
}

func (e *AgentError) Error() string {
	if e.Cause != nil {
		return e.Code + ": " + e.Message + " (caused by: " + e.Cause.Error() + ")"
	}
	return e.Code + ": " + e.Message
}

// NewAgentError creates a new agent error.
func NewAgentError(code, message string) *AgentError {
	return &AgentError{Code: code, Message: message}
}

// WrapAgentError wraps an existing error.
func WrapAgentError(code string, cause error) *AgentError {
	return &AgentError{Code: code, Message: cause.Error(), Cause: cause}
}
