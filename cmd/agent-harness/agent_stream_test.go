package main

import (
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func TestStreamedTextMatchesOnlyTheSameProse(t *testing.T) {
	tests := []struct {
		name     string
		message  types.Message
		streamed string
		want     bool
	}{
		{
			name:     "exact match",
			message:  types.Message{Content: []types.ContentBlock{types.TextBlock{Text: "Hello world"}}},
			streamed: "Hello world",
			want:     true,
		},
		{
			name: "fragmented blocks concatenate",
			message: types.Message{Content: []types.ContentBlock{
				types.TextBlock{Text: "Hello"},
				types.ToolUseBlock{ID: "t1", Name: "bash"},
				types.TextBlock{Text: " world"},
			}},
			streamed: "Hello world",
			want:     true,
		},
		{
			name:     "no deltas is a fallback, not a duplicate",
			message:  types.Message{Content: []types.ContentBlock{types.TextBlock{Text: "Hello"}}},
			streamed: "",
			want:     false,
		},
		{
			name:     "different prose is not suppressed",
			message:  types.Message{Content: []types.ContentBlock{types.TextBlock{Text: "Hello world"}}},
			streamed: "Hello",
			want:     false,
		},
		{
			name:     "tool-only message has no prose to suppress",
			message:  types.Message{Content: []types.ContentBlock{types.ToolUseBlock{ID: "t1", Name: "bash"}}},
			streamed: "",
			want:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := streamedTextMatches(tt.message, tt.streamed); got != tt.want {
				t.Fatalf("streamedTextMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}
