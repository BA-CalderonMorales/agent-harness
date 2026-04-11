package buckets

import (
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/permissions"
	"github.com/BA-CalderonMorales/agent-harness/internal/tools"
)

// UserSettingsBucket handles user-level permission rules.
// It implements PermissionBase for user configuration.
type UserSettingsBucket struct {
	rules []permissions.PermissionRule
}

// UserSettings creates a new user settings bucket.
func UserSettings(rules ...permissions.PermissionRule) *UserSettingsBucket {
	return &UserSettingsBucket{
		rules: rules,
	}
}

// Name returns the bucket identifier.
func (u *UserSettingsBucket) Name() string {
	return "usersettings"
}

// CanHandle determines if this bucket handles the permission type.
func (u *UserSettingsBucket) CanHandle(toolName string, permType permissions.PermissionType) bool {
	// User settings can handle all permission types
	return true
}

// Capabilities describes what this bucket can do.
func (u *UserSettingsBucket) Capabilities() permissions.PermissionBucketCapabilities {
	return permissions.PermissionBucketCapabilities{
		IsLayered:   true,
		IsOverride:  true,
		RuleSources: []string{"userSettings"},
		Category:    "usersettings",
		Priority:    10, // Low priority - user settings are foundational
	}
}

// GetRules returns the rules this bucket provides.
func (u *UserSettingsBucket) GetRules() []permissions.PermissionRule {
	return u.rules
}

// Evaluate runs the permission check against user settings.
func (u *UserSettingsBucket) Evaluate(ctx permissions.PermissionEvaluationContext) tools.PermissionDecision {
	// Check deny rules first
	if denyRules, ok := ctx.PermCtx.AlwaysDenyRules[permissions.SourceUserSettings]; ok {
		for _, rule := range denyRules {
			if u.ruleMatches(ctx.Tool, ctx.Input, rule) {
				return tools.PermissionDecision{
					Behavior:       tools.Deny,
					Message:        "Denied by user settings",
					DecisionReason: string(rule.Source),
				}
			}
		}
	}
	
	// Check allow rules
	if allowRules, ok := ctx.PermCtx.AlwaysAllowRules[permissions.SourceUserSettings]; ok {
		for _, rule := range allowRules {
			if u.ruleMatches(ctx.Tool, ctx.Input, rule) {
				return tools.PermissionDecision{
					Behavior:       tools.Allow,
					UpdatedInput:   ctx.Input,
					DecisionReason: string(rule.Source),
				}
			}
		}
	}
	
	// Check ask rules
	if askRules, ok := ctx.PermCtx.AlwaysAskRules[permissions.SourceUserSettings]; ok {
		for _, rule := range askRules {
			if u.ruleMatches(ctx.Tool, ctx.Input, rule) {
				return tools.PermissionDecision{
					Behavior:       tools.Ask,
					Message:        "Ask by user settings",
					UpdatedInput:   ctx.Input,
					DecisionReason: string(rule.Source),
				}
			}
		}
	}
	
	// No matching rule - passthrough
	return tools.PermissionDecision{
		Behavior:     tools.Passthrough,
		UpdatedInput: ctx.Input,
	}
}

// ruleMatches checks if a rule applies to a tool+input pair.
func (u *UserSettingsBucket) ruleMatches(tool tools.Tool, input map[string]any, rule permissions.PermissionRule) bool {
	// Tool name match (including aliases)
	if !strings.EqualFold(rule.ToolName, tool.Name) {
		matched := false
		for _, alias := range tool.Aliases {
			if strings.EqualFold(rule.ToolName, alias) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	
	// Blanket rule (no content restriction)
	if rule.RuleContent == "" {
		return true
	}
	
	// Content-level match using tool's permission matcher
	if tool.PreparePermissionMatcher != nil {
		matcher := tool.PreparePermissionMatcher(input)
		return matcher(rule.RuleContent)
	}
	
	// Fallback: simple wildcard on first string field
	for _, v := range input {
		if s, ok := v.(string); ok {
			if u.matchWildcard(rule.RuleContent, s) {
				return true
			}
		}
	}
	return false
}

// matchWildcard implements gitignore-style wildcard matching.
func (u *UserSettingsBucket) matchWildcard(pattern, text string) bool {
	if pattern == "*" {
		return true
	}
	// Use filepath.Match for basic glob behavior
	// Note: we're using path/filepath but the import isn't shown in snippet
	// Simple implementation:
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(text, prefix)
	}
	return pattern == text
}

// Ensure UserSettingsBucket implements PermissionBase
var _ permissions.PermissionBase = (*UserSettingsBucket)(nil)
