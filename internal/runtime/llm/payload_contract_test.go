package llm

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func TestBuildPayloadHandlesNilToolInputEmptyAndMultipleToolResults(t *testing.T) {
	client := NewHTTPClient("openai", "sk-test")
	req := Request{
		Model: "gpt-4o",
		Messages: []types.Message{
			{
				Role: types.RoleAssistant,
				Content: []types.ContentBlock{
					types.ToolUseBlock{ID: "call_nil", Name: "list_dir", Input: nil},
				},
			},
			{
				Role: types.RoleUser,
				Content: []types.ContentBlock{
					types.ToolResultBlock{ToolUseID: "call_nil", Content: ""},
					types.ToolResultBlock{ToolUseID: "call_two", Content: "ok"},
				},
			},
		},
	}

	raw, err := client.buildPayload(req)
	if err != nil {
		t.Fatalf("buildPayload() error = %v", err)
	}
	var wire struct {
		Messages []struct {
			Role       string `json:"role"`
			ToolCallID string `json:"tool_call_id"`
			Content    any    `json:"content"`
			ToolCalls  []struct {
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if len(wire.Messages) != 3 {
		t.Fatalf("wire messages = %d, want 3 (assistant + 2 tool results): %s", len(wire.Messages), string(raw))
	}
	if got := wire.Messages[0].ToolCalls[0].Function.Arguments; got != "{}" {
		t.Fatalf("nil input marshaled as %q, want {}", got)
	}
	if wire.Messages[1].Role != "tool" || wire.Messages[1].ToolCallID != "call_nil" || wire.Messages[1].Content != " " {
		t.Fatalf("empty tool result = %+v, want role=tool content=' '", wire.Messages[1])
	}
	if wire.Messages[2].Role != "tool" || wire.Messages[2].ToolCallID != "call_two" || wire.Messages[2].Content != "ok" {
		t.Fatalf("second tool result = %+v, want role=tool content='ok'", wire.Messages[2])
	}
}

func TestReadSSERejectsOversizedLine(t *testing.T) {
	oversized := "data: " + strings.Repeat("a", maxSSELineBytes+16)
	got := observeSSEEvents(collectSSEEvents(t, context.Background(), io.NopCloser(strings.NewReader(oversized))))
	if len(got.errors) != 1 {
		t.Fatalf("errors = %v, want 1 oversized-line error", got.errors)
	}
	assertErrorContains(t, got.errors[0], "SSE line exceeds")
}
