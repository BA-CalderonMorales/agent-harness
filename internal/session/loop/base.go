// Package loop provides the modular agent loop architecture.
//
// The loop is decomposed into focused interfaces that can be implemented
// independently by "buckets" - domain-specific loop implementations.
//
// Core principle: Each bucket implements LoopBase but only handles
// what it knows. The LoopOrchestrator coordinates without digging
// into bucket internals.
package loop

import (
	"context"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// LoopBase is the fundamental interface all loop buckets must implement.
// It defines the contract for participating in the agent loop without
// prescribing implementation details.
type LoopBase interface {
	// Name returns the bucket identifier (e.g., "filesystem", "shell")
	Name() string

	// CanHandle determines if this bucket should handle the given tool call.
	// This allows the orchestrator to route tool calls to appropriate buckets.
	CanHandle(toolName string, input map[string]any) bool

	// Execute runs the tool call and returns results.
	// The bucket handles all internals: validation, permissions, execution.
	Execute(ctx ExecutionContext) LoopResult

	// Capabilities describes what this bucket can do.
	Capabilities() BucketCapabilities
}

// ExecutionContext carries all context needed for a single tool execution.
// Buckets receive this instead of digging into global state.
type ExecutionContext struct {
	Context    context.Context
	ToolName   string
	Input      map[string]any
	ToolUseID  string
	Messages   []types.Message
	CanUseTool tools.CanUseToolFn
	OnProgress tools.OnProgress
}

// LoopResult is the standardized output from any bucket execution.
type LoopResult struct {
	Success    bool
	Data       any
	Messages   []types.Message
	Error      LoopError
	ShouldHalt bool // If true, stop the entire loop
	Retryable  bool // If true and failed, can retry
}

// BucketCapabilities describes static capabilities of a bucket.
type BucketCapabilities struct {
	IsConcurrencySafe bool     // Can run alongside other safe buckets
	IsReadOnly        bool     // Doesn't modify state
	IsDestructive     bool     // Can destroy/modify state
	ToolNames         []string // Tools this bucket handles
	Category          string   // "filesystem", "shell", "search", etc.
}

// LoopState tracks the current state of a loop iteration.
type LoopState struct {
	TurnCount                    int
	Messages                     []types.Message
	MaxOutputTokensOverride      int
	MaxOutputTokensRecoveryCount int
	HasAttemptedReactiveCompact  bool
	ToolUseContext               tools.Context
}

// NewLoopState creates a fresh loop state.
func NewLoopState() *LoopState {
	return &LoopState{
		TurnCount: 1,
		Messages:  make([]types.Message, 0),
		ToolUseContext: tools.Context{
			ToolDecisions:           make(map[string]tools.ToolDecision),
			LoadedNestedMemoryPaths: make(map[string]struct{}),
			DiscoveredSkillNames:    make(map[string]struct{}),
		},
	}
}

// QueryParams configures a single agentic turn.
type QueryParams struct {
	Messages        []types.Message
	SystemPrompt    string
	UserContext     map[string]string
	SystemContext   map[string]string
	CanUseTool      tools.CanUseToolFn
	ToolUseContext  tools.Context
	FallbackModel   string
	QuerySource     types.QuerySource
	MaxOutputTokens int
	MaxTurns        int
	SkipCacheWrite  bool
}
