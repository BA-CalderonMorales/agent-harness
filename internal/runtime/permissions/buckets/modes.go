package buckets

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/permissions"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
)

// ModeBucket handles mode-based permission transformations.
// It implements PermissionBase for dontAsk, auto, bypass modes.
type ModeBucket struct{}

// Mode creates a new mode transformation bucket.
func Mode() *ModeBucket {
	return &ModeBucket{}
}

// Name returns the bucket identifier.
func (m *ModeBucket) Name() string {
	return "mode"
}

// CanHandle determines if this bucket handles the permission type.
func (m *ModeBucket) CanHandle(toolName string, permType permissions.PermissionType) bool {
	// Mode bucket handles all permission types
	return true
}

// Capabilities describes what this bucket can do.
func (m *ModeBucket) Capabilities() permissions.PermissionBucketCapabilities {
	return permissions.PermissionBucketCapabilities{
		IsLayered:   false,
		IsOverride:  true,
		RuleSources: []string{"mode"},
		Category:    "mode",
		Priority:    50, // Medium priority - applies after specific rules
	}
}

// GetRules returns empty - mode bucket doesn't use rules.
func (m *ModeBucket) GetRules() []permissions.PermissionRule {
	return nil
}

// Evaluate applies mode-based transformations.
func (m *ModeBucket) Evaluate(ctx permissions.PermissionEvaluationContext) tools.PermissionDecision {
	switch ctx.Mode {
	case permissions.ModeDontAsk:
		// In dontAsk mode, most tools are auto-allowed unless destructive
		if ctx.Tool.Capabilities.IsDestructive != nil && ctx.Tool.Capabilities.IsDestructive(ctx.Input) {
			return tools.PermissionDecision{
				Behavior:     tools.Ask,
				UpdatedInput: ctx.Input,
				Message:      "Destructive operation requires confirmation in dontAsk mode",
			}
		}
		return tools.PermissionDecision{
			Behavior:       tools.Allow,
			UpdatedInput:   ctx.Input,
			DecisionReason: "dontAsk_mode",
		}
		
	case permissions.ModeBypassPermissions:
		if ctx.PermCtx.IsBypassPermissionsAvailable {
			return tools.PermissionDecision{
				Behavior:       tools.Allow,
				UpdatedInput:   ctx.Input,
				DecisionReason: "bypass_permissions",
			}
		}
		return tools.PermissionDecision{
			Behavior:     tools.Ask,
			UpdatedInput: ctx.Input,
			Message:      "Bypass permissions not available",
		}
		
	case permissions.ModeAuto:
		// Auto mode: defer to classifier (returns ask to trigger classification)
		return tools.PermissionDecision{
			Behavior:     tools.Ask,
			UpdatedInput: ctx.Input,
			Message:      "Auto mode requires classification",
		}
		
	case permissions.ModeAcceptEdits:
		// Auto-allow edit/write tools
		if ctx.Tool.Name == "edit" || ctx.Tool.Name == "write" {
			return tools.PermissionDecision{
				Behavior:       tools.Allow,
				UpdatedInput:   ctx.Input,
				DecisionReason: "accept_edits_mode",
			}
		}
		return tools.PermissionDecision{
			Behavior:     tools.Passthrough,
			UpdatedInput: ctx.Input,
		}
		
	case permissions.ModePlan:
		// Plan mode: ask for everything
		return tools.PermissionDecision{
			Behavior:     tools.Ask,
			UpdatedInput: ctx.Input,
			Message:      "Plan mode - all operations require approval",
		}
		
	default:
		// Default mode: passthrough to tool-specific checks
		return tools.PermissionDecision{
			Behavior:     tools.Passthrough,
			UpdatedInput: ctx.Input,
		}
	}
}

// Ensure ModeBucket implements PermissionBase
var _ permissions.PermissionBase = (*ModeBucket)(nil)
