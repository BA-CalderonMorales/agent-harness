package permissions

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/tools"
)

// Mode describes the current permission behavior.
type Mode string

const (
	ModeDefault            Mode = "default"
	ModeAcceptEdits        Mode = "acceptEdits"
	ModeDontAsk            Mode = "dontAsk"
	ModePlan               Mode = "plan"
	ModeAuto               Mode = "auto"
	ModeBypassPermissions  Mode = "bypassPermissions"
	ModeBubble             Mode = "bubble"
)

// RuleSource identifies where a permission rule came from.
type RuleSource string

const (
	SourceUserSettings    RuleSource = "userSettings"
	SourceProjectSettings RuleSource = "projectSettings"
	SourceLocalSettings   RuleSource = "localSettings"
	SourceFlagSettings    RuleSource = "flagSettings"
	SourcePolicySettings  RuleSource = "policySettings"
	SourceCliArg          RuleSource = "cliArg"
	SourceCommand         RuleSource = "command"
	SourceSession         RuleSource = "session"
)

// PermissionRule is a single allow/deny/ask rule.
type PermissionRule struct {
	ToolName    string
	RuleContent string // e.g., "git *"
	Behavior    tools.DecisionBehavior
	Source      RuleSource
}

// Context is the immutable permission context.
type Context struct {
	Mode                        Mode
	AdditionalWorkingDirs       map[string]string
	AlwaysAllowRules            map[RuleSource][]PermissionRule
	AlwaysDenyRules             map[RuleSource][]PermissionRule
	AlwaysAskRules              map[RuleSource][]PermissionRule
	IsBypassPermissionsAvailable bool
	ShouldAvoidPrompts          bool
	AwaitAutomatedChecks        bool
	PrePlanMode                 Mode
}

// EmptyContext returns a safe default context.
func EmptyContext() Context {
	return Context{
		Mode:                        ModeDefault,
		AdditionalWorkingDirs:       make(map[string]string),
		AlwaysAllowRules:            make(map[RuleSource][]PermissionRule),
		AlwaysDenyRules:             make(map[RuleSource][]PermissionRule),
		AlwaysAskRules:              make(map[RuleSource][]PermissionRule),
		IsBypassPermissionsAvailable: false,
	}
}

// Evaluate runs the layered permission decision stack.
func Evaluate(tool tools.Tool, input map[string]any, permCtx Context) tools.PermissionDecision {
	// Layer 1: blanket deny rules
	if rule := findMatchingRule(tool, input, permCtx.AlwaysDenyRules); rule != nil {
		return tools.PermissionDecision{
			Behavior:       tools.Deny,
			Message:        fmt.Sprintf("Denied by rule from %s", rule.Source),
			DecisionReason: string(rule.Source),
		}
	}

	// Layer 2: blanket allow rules
	if rule := findMatchingRule(tool, input, permCtx.AlwaysAllowRules); rule != nil {
		return tools.PermissionDecision{
			Behavior:       tools.Allow,
			UpdatedInput:   input,
			DecisionReason: string(rule.Source),
		}
	}

	// Layer 3: always ask rules
	if rule := findMatchingRule(tool, input, permCtx.AlwaysAskRules); rule != nil {
		return tools.PermissionDecision{
			Behavior:       tools.Ask,
			Message:        fmt.Sprintf("Ask by rule from %s", rule.Source),
			UpdatedInput:   input,
			DecisionReason: string(rule.Source),
		}
	}

	// Layer 4: mode transformations
	switch permCtx.Mode {
	case ModeDontAsk:
		// In dontAsk mode, most tools are auto-allowed unless destructive
		if tool.Capabilities.IsDestructive != nil && tool.Capabilities.IsDestructive(input) {
			return tools.PermissionDecision{Behavior: tools.Ask, UpdatedInput: input}
		}
		return tools.PermissionDecision{Behavior: tools.Allow, UpdatedInput: input}
	case ModeBypassPermissions:
		if permCtx.IsBypassPermissionsAvailable {
			return tools.PermissionDecision{Behavior: tools.Allow, UpdatedInput: input}
		}
		return tools.PermissionDecision{Behavior: tools.Ask, UpdatedInput: input}
	case ModeAuto:
		// Auto mode: defer to classifier (handled upstream)
		return tools.PermissionDecision{
			Behavior:     tools.Ask,
			UpdatedInput: input,
			Message:      "Auto mode requires classification",
		}
	}

	// Layer 5: tool-specific check
	if tool.CheckPermissions != nil {
		return tool.CheckPermissions(input, tools.Context{})
	}

	// Default: ask
	return tools.PermissionDecision{Behavior: tools.Ask, UpdatedInput: input}
}

// findMatchingRule scans rules for a match.
func findMatchingRule(tool tools.Tool, input map[string]any, rulesBySource map[RuleSource][]PermissionRule) *PermissionRule {
	for _, rules := range rulesBySource {
		for _, rule := range rules {
			if ruleMatches(tool, input, rule) {
				return &rule
			}
		}
	}
	return nil
}

// ruleMatches checks if a rule applies to a tool+input pair.
func ruleMatches(tool tools.Tool, input map[string]any, rule PermissionRule) bool {
	// Tool name match (including MCP prefix normalization)
	toolName := tool.Name
	if !strings.EqualFold(rule.ToolName, toolName) {
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
			if matchWildcard(rule.RuleContent, s) {
				return true
			}
		}
	}
	return false
}

// matchWildcard implements gitignore-style wildcard matching.
func matchWildcard(pattern, text string) bool {
	// Simplified wildcard: * matches any sequence, ? matches one character
	if pattern == "*" {
		return true
	}
	// Use filepath.Match for basic glob behavior
	matched, _ := filepath.Match(pattern, text)
	return matched
}

// DenialTrackingState tracks consecutive denials for auto-mode fallback.
type DenialTrackingState struct {
	ConsecutiveDenials int
	ShouldFallbackToPrompting bool
}

// RecordDenial increments the denial counter.
func (d *DenialTrackingState) RecordDenial() {
	d.ConsecutiveDenials++
	if d.ConsecutiveDenials >= 3 {
		d.ShouldFallbackToPrompting = true
	}
}

// RecordAllow resets the denial counter.
func (d *DenialTrackingState) RecordAllow() {
	d.ConsecutiveDenials = 0
	d.ShouldFallbackToPrompting = false
}
