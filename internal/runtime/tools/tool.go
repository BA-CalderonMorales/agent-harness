package tools

import (
	"context"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// ValidationResult is the output of tool-specific input validation.
type ValidationResult struct {
	Valid     bool
	Message   string
	ErrorCode int
}

// ToolResult wraps the output of a tool call.
type ToolResult struct {
	Data            any
	NewMessages     []types.Message
	ContextModifier func(ctx Context) Context
}

// Progress represents a progress update from a tool.
type Progress struct {
	ToolUseID string
	Data      any
}

// OnProgress is the callback signature for tool progress.
type OnProgress func(data any)

// CanUseToolFn is the permission checker injected by the agent loop.
type CanUseToolFn func(toolName string, input map[string]any, ctx Context) (PermissionDecision, error)

// PermissionDecision is the result of a permission check.
type PermissionDecision struct {
	Behavior       DecisionBehavior
	UpdatedInput   map[string]any
	Message        string
	DecisionReason string
}

// DecisionBehavior enumerates permission outcomes.
type DecisionBehavior string

const (
	Allow       DecisionBehavior = "allow"
	Deny        DecisionBehavior = "deny"
	Ask         DecisionBehavior = "ask"
	Passthrough DecisionBehavior = "passthrough"
)

// CapabilityFlags describes static/dynamic tool capabilities.
type CapabilityFlags struct {
	IsEnabled               func() bool
	IsConcurrencySafe       func(input map[string]any) bool
	IsReadOnly              func(input map[string]any) bool
	IsDestructive           func(input map[string]any) bool
	InterruptBehavior       func() string // "cancel" or "block"
	RequiresUserInteraction func() bool
	IsSearchOrReadCommand   func(input map[string]any) SearchReadFlags
	IsOpenWorld             func(input map[string]any) bool
	IsTransparentWrapper    func() bool
}

// SearchReadFlags is returned by IsSearchOrReadCommand.
type SearchReadFlags struct {
	IsSearch bool
	IsRead   bool
	IsList   bool
}

// Options is the static configuration passed to tools.
type Options struct {
	Commands                []Command
	Debug                   bool
	MainLoopModel           string
	Tools                   []Tool
	Verbose                 bool
	IsNonInteractiveSession bool
	AgentDefinitions        AgentDefinitions
	MaxBudgetUsd            float64
	CustomSystemPrompt      string
	AppendSystemPrompt      string
}

// Command represents a slash command definition.
type Command struct {
	Name        string
	Description string
	Handler     func(args []string) error
}

// AgentDefinitions holds agent configuration.
type AgentDefinitions struct {
	ActiveAgents      []AgentDef
	AllowedAgentTypes []string
}

// AgentDef describes a sub-agent template.
type AgentDef struct {
	Name        string
	Type        string
	Description string
	Prompt      string
}

// FileReadingLimits caps file read operations.
type FileReadingLimits struct {
	MaxTokens    int
	MaxSizeBytes int64
}

// GlobLimits caps glob operations.
type GlobLimits struct {
	MaxResults int
}

// Context carries mutable execution state for a tool call.
type Context struct {
	Options                 Options
	AbortController         context.Context
	GetAppState             func() any
	SetAppState             func(updater func(prev any) any)
	SetAppStateForTasks     func(updater func(prev any) any)
	Messages                []types.Message
	ToolUseID               string
	AgentID                 string
	AgentType               string
	QueryTracking           QueryChainTracking
	FileReadingLimits       FileReadingLimits
	GlobLimits              GlobLimits
	ToolDecisions           map[string]ToolDecision
	ContentReplacement      any
	LoadedNestedMemoryPaths map[string]struct{}
	DiscoveredSkillNames    map[string]struct{}
	LocalDenialTracking     any
	PreserveToolUseResults  bool
	RequireCanUseTool       bool
	RenderedSystemPrompt    string
	// SubAgentQuery executes a sub-agent query with fresh context.
	// If nil, sub-agent spawning falls back to placeholder behavior.
	SubAgentQuery func(prompt string) (string, error)
}

// QueryChainTracking prevents infinite recursion.
type QueryChainTracking struct {
	ChainID string
	Depth   int
}

// ToolDecision records a previous user decision about a tool.
type ToolDecision struct {
	Source    string
	Decision  string // "accept" or "reject"
	Timestamp int64
}

// Tool is the core descriptor object pattern for agent tools.
// In Go, we use a struct with function fields rather than a fat interface.
type Tool struct {
	Name               string
	Aliases            []string
	SearchHint         string
	Description        string
	InputSchema        func() map[string]any
	InputJSONSchema    map[string]any
	OutputSchema       func() map[string]any
	MaxResultSizeChars int64

	// Capabilities
	Capabilities CapabilityFlags

	// Lifecycle
	ValidateInput    func(input map[string]any, ctx Context) ValidationResult
	CheckPermissions func(input map[string]any, ctx Context) PermissionDecision
	Call             func(input map[string]any, ctx Context, canUseTool CanUseToolFn, onProgress OnProgress) (ToolResult, error)
	MapResult        func(result any, toolUseID string) types.ToolResultBlock

	// Rendering / UI (optional)
	UserFacingName         func(input map[string]any) string
	GetToolUseSummary      func(input map[string]any) string
	GetActivityDescription func(input map[string]any) string
	ToAutoClassifierInput  func(input map[string]any) any

	// Input pipeline hooks
	BackfillObservableInput  func(input map[string]any)
	PreparePermissionMatcher func(input map[string]any) func(pattern string) bool
}

// DefaultCapabilityFlags returns safe defaults (fail-closed).
func DefaultCapabilityFlags() CapabilityFlags {
	return CapabilityFlags{
		IsEnabled:               func() bool { return true },
		IsConcurrencySafe:       func(map[string]any) bool { return false },
		IsReadOnly:              func(map[string]any) bool { return false },
		IsDestructive:           func(map[string]any) bool { return false },
		InterruptBehavior:       func() string { return "block" },
		RequiresUserInteraction: func() bool { return false },
		IsSearchOrReadCommand:   func(map[string]any) SearchReadFlags { return SearchReadFlags{} },
		IsOpenWorld:             func(map[string]any) bool { return false },
		IsTransparentWrapper:    func() bool { return false },
	}
}

// NewTool creates a tool with safe defaults merged over the provided fields.
func NewTool(t Tool) Tool {
	if t.Capabilities.IsEnabled == nil {
		t.Capabilities.IsEnabled = DefaultCapabilityFlags().IsEnabled
	}
	if t.Capabilities.IsConcurrencySafe == nil {
		t.Capabilities.IsConcurrencySafe = DefaultCapabilityFlags().IsConcurrencySafe
	}
	if t.Capabilities.IsReadOnly == nil {
		t.Capabilities.IsReadOnly = DefaultCapabilityFlags().IsReadOnly
	}
	if t.Capabilities.IsDestructive == nil {
		t.Capabilities.IsDestructive = DefaultCapabilityFlags().IsDestructive
	}
	if t.Capabilities.InterruptBehavior == nil {
		t.Capabilities.InterruptBehavior = DefaultCapabilityFlags().InterruptBehavior
	}
	if t.Capabilities.RequiresUserInteraction == nil {
		t.Capabilities.RequiresUserInteraction = DefaultCapabilityFlags().RequiresUserInteraction
	}
	if t.Capabilities.IsSearchOrReadCommand == nil {
		t.Capabilities.IsSearchOrReadCommand = DefaultCapabilityFlags().IsSearchOrReadCommand
	}
	if t.Capabilities.IsOpenWorld == nil {
		t.Capabilities.IsOpenWorld = DefaultCapabilityFlags().IsOpenWorld
	}
	if t.Capabilities.IsTransparentWrapper == nil {
		t.Capabilities.IsTransparentWrapper = DefaultCapabilityFlags().IsTransparentWrapper
	}
	if t.CheckPermissions == nil {
		t.CheckPermissions = func(input map[string]any, ctx Context) PermissionDecision {
			return PermissionDecision{Behavior: Allow, UpdatedInput: input}
		}
	}
	if t.UserFacingName == nil {
		t.UserFacingName = func(map[string]any) string { return t.Name }
	}
	if t.ToAutoClassifierInput == nil {
		t.ToAutoClassifierInput = func(map[string]any) any { return "" }
	}
	if t.MaxResultSizeChars == 0 {
		t.MaxResultSizeChars = 10000
	}
	return t
}

// ToolRegistry manages the collection of available tools.
type ToolRegistry struct {
	builtIns []Tool
	mcpTools []Tool
}

// NewRegistry creates an empty registry.
func NewRegistry() *ToolRegistry {
	return &ToolRegistry{}
}

// RegisterBuiltIn adds a built-in tool.
func (r *ToolRegistry) RegisterBuiltIn(t Tool) {
	r.builtIns = append(r.builtIns, t)
}

// RegisterMCP adds an MCP-provided tool.
func (r *ToolRegistry) RegisterMCP(t Tool) {
	r.mcpTools = append(r.mcpTools, t)
}

// FindToolByName locates a tool by primary name or alias.
func (r *ToolRegistry) FindToolByName(name string) (Tool, bool) {
	for _, t := range r.AllTools() {
		if t.Name == name {
			return t, true
		}
		for _, a := range t.Aliases {
			if a == name {
				return t, true
			}
		}
	}
	return Tool{}, false
}

// AllTools returns the combined tool list with built-ins first.
func (r *ToolRegistry) AllTools() []Tool {
	// Keep partitions separate for cache stability, then concat
	out := make([]Tool, 0, len(r.builtIns)+len(r.mcpTools))
	out = append(out, r.builtIns...)
	out = append(out, r.mcpTools...)
	return out
}

// FilterEnabled returns only enabled tools.
func (r *ToolRegistry) FilterEnabled() []Tool {
	all := r.AllTools()
	out := make([]Tool, 0, len(all))
	for _, t := range all {
		if t.Capabilities.IsEnabled != nil && t.Capabilities.IsEnabled() {
			out = append(out, t)
		}
	}
	return out
}
