package behaviors

import (
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/permissions"
	"github.com/BA-CalderonMorales/agent-harness/internal/testharness"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func TestBehavior_LoopHappyPaths(t *testing.T) {
	tests := []struct {
		name       string
		given      func(*testharness.Fixture)
		mockEvents []types.LLMEvent
		wantTypes  []string
	}{
		{
			name: "B8: text response yields StreamMessage",
			given: func(f *testharness.Fixture) {
				f.SetPermissionMode(permissions.ModeDontAsk)
				f.MockLLM.Events = llm.MockTextResponse("Hello, world!")
			},
			wantTypes: []string{"StreamRequestStart", "StreamMessage"},
		},
		{
			name: "B9: tool use response yields ToolUseBlock",
			given: func(f *testharness.Fixture) {
				f.SetPermissionMode(permissions.ModeDontAsk)
				f.MockLLM.Events = llm.MockToolUseResponse("bash", "echo hi")
			},
			wantTypes: []string{"StreamRequestStart", "StreamMessage", "ToolUseBlock"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := testharness.NewFixture(t)
			if tt.given != nil {
				tt.given(f)
			}

			events := f.QueryLoop(nil, "You are a test assistant.")

			gotTypes := eventTypes(events)
			for _, want := range tt.wantTypes {
				if !sliceContains(gotTypes, want) {
					t.Errorf("expected at least one %s event, got %v", want, gotTypes)
				}
			}
		})
	}
}

func sliceContains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func eventTypes(events []types.StreamEvent) []string {
	var result []string
	for _, ev := range events {
		switch e := ev.(type) {
		case types.StreamRequestStart:
			result = append(result, "StreamRequestStart")
		case types.StreamMessage:
			result = append(result, "StreamMessage")
			for _, block := range e.Message.Content {
				switch block.(type) {
				case types.ToolUseBlock:
					result = append(result, "ToolUseBlock")
				case types.ToolResultBlock:
					result = append(result, "ToolResultBlock")
				}
			}
		case types.StreamError:
			result = append(result, "StreamError")
		default:
			result = append(result, "Unknown")
		}
	}
	return result
}
