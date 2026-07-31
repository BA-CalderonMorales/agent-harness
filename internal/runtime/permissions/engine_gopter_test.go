package permissions

import (
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestPermissionsEnginePropertyBased(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MaxSize = 50
	properties := gopter.NewProperties(parameters)

	// Invariant 1: RecordAllow always resets DenialTrackingState counters
	properties.Property("DenialTrackingState: RecordAllow resets counters", prop.ForAll(
		func(denials int) bool {
			state := &DenialTrackingState{}
			for i := 0; i < denials; i++ {
				state.RecordDenial()
			}
			state.RecordAllow()
			return state.ConsecutiveDenials == 0 && !state.ShouldFallbackToPrompting
		},
		gen.IntRange(0, 100),
	))

	// Invariant 2: RecordDenial threshold behavior
	properties.Property("DenialTrackingState: RecordDenial threshold at 3", prop.ForAll(
		func(denials int) bool {
			state := &DenialTrackingState{}
			for i := 0; i < denials; i++ {
				state.RecordDenial()
			}
			expectedFallback := denials >= 3
			return state.ConsecutiveDenials == denials && state.ShouldFallbackToPrompting == expectedFallback
		},
		gen.IntRange(1, 50),
	))

	// Invariant 3: AlwaysDeny rules always take precedence over AlwaysAllow rules
	properties.Property("Evaluate: AlwaysDeny takes precedence over AlwaysAllow", prop.ForAll(
		func(toolName string) bool {
			if toolName == "" {
				return true
			}
			t := tools.Tool{Name: toolName}
			rule := PermissionRule{ToolName: toolName, Behavior: tools.Deny, Source: SourceUserSettings}
			allowRule := PermissionRule{ToolName: toolName, Behavior: tools.Allow, Source: SourceUserSettings}

			ctx := EmptyContext()
			ctx.AlwaysDenyRules[SourceUserSettings] = []PermissionRule{rule}
			ctx.AlwaysAllowRules[SourceUserSettings] = []PermissionRule{allowRule}

			decision := Evaluate(t, nil, ctx)
			return decision.Behavior == tools.Deny
		},
		gen.AlphaString(),
	))

	// Invariant 4: AlwaysAllow rules take precedence over default Ask
	properties.Property("Evaluate: AlwaysAllow produces Allow decision when no Deny rule exists", prop.ForAll(
		func(toolName string) bool {
			if toolName == "" {
				return true
			}
			t := tools.Tool{Name: toolName}
			allowRule := PermissionRule{ToolName: toolName, Behavior: tools.Allow, Source: SourceUserSettings}

			ctx := EmptyContext()
			ctx.AlwaysAllowRules[SourceUserSettings] = []PermissionRule{allowRule}

			decision := Evaluate(t, nil, ctx)
			return decision.Behavior == tools.Allow
		},
		gen.AlphaString(),
	))

	// Invariant 5: matchWildcard "*" matches any text
	properties.Property("matchWildcard: * matches any input text", prop.ForAll(
		func(text string) bool {
			return matchWildcard("*", text)
		},
		gen.AnyString(),
	))

	// Invariant 6: matchWildcard exact string matches itself
	properties.Property("matchWildcard: exact string match", prop.ForAll(
		func(text string) bool {
			// Avoid characters special to filepath.Match (like '*', '?', '[')
			sanitized := ""
			for _, r := range text {
				if r != '*' && r != '?' && r != '[' && r != ']' && r != '\\' {
					sanitized += string(r)
				}
			}
			if sanitized == "" {
				return true
			}
			return matchWildcard(sanitized, sanitized)
		},
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}
