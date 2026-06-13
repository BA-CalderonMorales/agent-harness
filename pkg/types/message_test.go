package types

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMessageJSONRoundTripRestoresContentBlockTypes(t *testing.T) {
	msg := Message{
		UUID:      "msg-1",
		Role:      RoleAssistant,
		Timestamp: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
		Content: []ContentBlock{
			TextBlock{Text: "hello"},
			ToolUseBlock{ID: "tool-1", Name: "bash", Input: map[string]any{"command": "pwd"}},
			ToolResultBlock{ToolUseID: "tool-1", Content: "/tmp/project", IsError: true},
			ThinkingBlock{Thinking: "private reasoning", Signature: "sig"},
		},
		APIError:   "boom",
		StopReason: "tool_use",
		Model:      "test-model",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.UUID != msg.UUID || decoded.Role != msg.Role || !decoded.Timestamp.Equal(msg.Timestamp) {
		t.Fatalf("decoded metadata mismatch: %#v", decoded)
	}
	if decoded.APIError != msg.APIError || decoded.StopReason != msg.StopReason || decoded.Model != msg.Model {
		t.Fatalf("decoded diagnostic fields mismatch: %#v", decoded)
	}
	if len(decoded.Content) != len(msg.Content) {
		t.Fatalf("decoded %d content blocks, want %d", len(decoded.Content), len(msg.Content))
	}

	if block, ok := decoded.Content[0].(TextBlock); !ok || block.Text != "hello" {
		t.Fatalf("content[0] = %#v, want TextBlock", decoded.Content[0])
	}
	if block, ok := decoded.Content[1].(ToolUseBlock); !ok || block.ID != "tool-1" || block.Name != "bash" || block.Input["command"] != "pwd" {
		t.Fatalf("content[1] = %#v, want ToolUseBlock", decoded.Content[1])
	}
	if block, ok := decoded.Content[2].(ToolResultBlock); !ok || block.ToolUseID != "tool-1" || block.Content != "/tmp/project" || !block.IsError {
		t.Fatalf("content[2] = %#v, want ToolResultBlock", decoded.Content[2])
	}
	if block, ok := decoded.Content[3].(ThinkingBlock); !ok || block.Thinking != "private reasoning" || block.Signature != "sig" {
		t.Fatalf("content[3] = %#v, want ThinkingBlock", decoded.Content[3])
	}
}

func TestMessageJSONDecodeDefaultsUnknownContentToTextBlock(t *testing.T) {
	data := []byte(`{
		"uuid": "msg-2",
		"role": "user",
		"content": [{"text": "fallback text", "extra": "ignored"}],
		"timestamp": "2026-05-01T12:00:00Z"
	}`)

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if len(decoded.Content) != 1 {
		t.Fatalf("decoded %d content blocks, want 1", len(decoded.Content))
	}
	block, ok := decoded.Content[0].(TextBlock)
	if !ok {
		t.Fatalf("content[0] = %#v, want TextBlock", decoded.Content[0])
	}
	if block.Text != "fallback text" {
		t.Fatalf("TextBlock.Text = %q, want fallback text", block.Text)
	}
}
