// Package permissions provides the modular permission architecture.
//
// Permissions are decomposed into focused interfaces that can be implemented
// independently by "buckets" - domain-specific permission rule sources.
//
// Core principle: Each bucket implements PermissionBase but only handles
// what it knows. The PermissionOrchestrator coordinates without digging
// into bucket internals.
package permissions

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
)

// PermissionBase is the fundamental interface all permission buckets must implement.
// It defines the contract for participating in permission evaluation without
// prescribing implementation details.
type PermissionBase interface {
	// Name returns the bucket identifier (e.g., "usersettings", "projectsettings", "cliargs")
	Name() string

	// CanHandle determines if this bucket should handle the given tool/permission check.
	// This allows the orchestrator to route permission checks to appropriate buckets.
	CanHandle(toolName string, permType PermissionType) bool

	// Evaluate runs the permission check and returns a decision.
	// The bucket handles all internals: rule matching, mode transformations.
	Evaluate(ctx PermissionEvaluationContext) tools.PermissionDecision

	// Capabilities describes what this bucket can do.
	Capabilities() PermissionBucketCapabilities

	// GetRules returns the rules this bucket provides.
	GetRules() []PermissionRule
}

// PermissionType categorizes permission operations.
type PermissionType string

const (
	PermTypeAllow PermissionType = "allow"
	PermTypeDeny  PermissionType = "deny"
	PermTypeAsk   PermissionType = "ask"
)

// PermissionEvaluationContext carries all context needed for a permission check.
// Buckets receive this instead of digging into global state.
type PermissionEvaluationContext struct {
	Tool          tools.Tool
	ToolName      string
	Input         map[string]any
	Mode          Mode
	PermCtx       Context
	PrevDecisions map[string]tools.ToolDecision
}

// PermissionBucketCapabilities describes static capabilities of a permission bucket.
type PermissionBucketCapabilities struct {
	IsLayered   bool     // Can have multiple rules that stack
	IsOverride  bool     // Rules can override other sources
	RuleSources []string // Sources this bucket handles
	Category    string   // "usersettings", "projectsettings", etc.
	Priority    int      // Evaluation priority (lower = first)
}

// PermissionOrchestrator coordinates multiple PermissionBase implementations.
// It routes permission checks to appropriate buckets without knowing their internals.
type PermissionOrchestrator struct {
	buckets []PermissionBase
}

// NewPermissionOrchestrator creates a new orchestrator with the given buckets.
func NewPermissionOrchestrator(buckets ...PermissionBase) *PermissionOrchestrator {
	return &PermissionOrchestrator{
		buckets: buckets,
	}
}

// RegisterBucket adds a bucket to the orchestrator.
func (o *PermissionOrchestrator) RegisterBucket(bucket PermissionBase) {
	o.buckets = append(o.buckets, bucket)
}

// Evaluate runs the full permission evaluation stack across all buckets.
func (o *PermissionOrchestrator) Evaluate(ctx PermissionEvaluationContext) tools.PermissionDecision {
	// Evaluate by priority order
	for _, bucket := range o.buckets {
		if !bucket.CanHandle(ctx.ToolName, PermTypeAllow) &&
			!bucket.CanHandle(ctx.ToolName, PermTypeDeny) &&
			!bucket.CanHandle(ctx.ToolName, PermTypeAsk) {
			continue
		}

		decision := bucket.Evaluate(ctx)

		// Deny is final
		if decision.Behavior == tools.Deny {
			return decision
		}

		// Allow can be overridden by later buckets
		if decision.Behavior == tools.Allow {
			// Continue to check for denies
			continue
		}

		// Ask means we need more info - continue
	}

	// Default: allow
	return tools.PermissionDecision{
		Behavior:     tools.Allow,
		UpdatedInput: ctx.Input,
	}
}

// GetAllRules returns all rules from all buckets.
func (o *PermissionOrchestrator) GetAllRules() []PermissionRule {
	var all []PermissionRule
	for _, bucket := range o.buckets {
		all = append(all, bucket.GetRules()...)
	}
	return all
}

// Types below are defined in engine.go:
// - Mode, ModeDefault, ModeAcceptEdits, ModeDontAsk, ModePlan, ModeAuto, ModeBypassPermissions, ModeBubble
// - RuleSource, SourceUserSettings, SourceProjectSettings, SourceLocalSettings, SourceFlagSettings, SourcePolicySettings, SourceCliArg, SourceCommand, SourceSession
// - PermissionRule
// - Context, EmptyContext
// - DenialTrackingState
