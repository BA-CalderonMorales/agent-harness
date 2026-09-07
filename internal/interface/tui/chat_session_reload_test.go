package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// These tests pin the 0.3.28 session-reload fix: after loading a saved
// session, tool rows rendered as bare carets ("▸ ✓  0.0s") because
// chatMessageFromSessionMessage set ToolName but never ToolDisplayName
// or ToolDetail — the collapse machinery merges on empty display names
// and renders nothing readable.

// TestSessionReloadToolRowHasDisplayName: a persisted bash tool call
// must round-trip to a rendered row containing the display name and the
// command detail, not a bare caret.
func TestSessionReloadToolRowHasDisplayName(t *testing.T) {
	m := NewChatModel()
	m.width = 120
	m.height = 24
	m.resize(120, 24)

	sessionMsgs := []types.Message{
		{
			UUID:      "u1",
			Role:      types.RoleUser,
			Content:   []types.ContentBlock{types.TextBlock{Text: "list the files"}},
			Timestamp: time.Now(),
		},
		{
			UUID: "t1",
			Role: types.RoleAssistant,
			Content: []types.ContentBlock{
				types.ToolUseBlock{ID: "call-1", Name: "bash", Input: map[string]any{"command": "ls -la"}},
				types.ToolResultBlock{ToolUseID: "call-1", Content: "file.txt"},
			},
			Timestamp: time.Now(),
		},
	}

	m.SetMessages(sessionMsgs)

	var toolMsg *ChatMessage
	for i := range m.messages {
		if m.messages[i].IsTool {
			toolMsg = &m.messages[i]
		}
	}
	if toolMsg == nil {
		t.Fatal("session reload produced no tool message")
	}
	if toolMsg.ToolDisplayName != "Shell" {
		t.Fatalf("ToolDisplayName = %q, want Shell", toolMsg.ToolDisplayName)
	}
	if toolMsg.ToolDetail != "ls -la" {
		t.Fatalf("ToolDetail = %q, want the command", toolMsg.ToolDetail)
	}
	if toolMsg.ToolStartedAt.IsZero() {
		t.Fatal("ToolStartedAt not carried from the persisted timestamp")
	}

	// The rendered row must name the tool and show the command.
	rendered, _, _ := m.renderSingleGroup(m.messages, indexTool(m.messages), m.toolsCollapsed)
	if !strings.Contains(rendered, "Shell") {
		t.Fatalf("rendered tool row missing display name:\n%s", rendered)
	}
	if !strings.Contains(rendered, "ls -la") {
		t.Fatalf("rendered tool row missing command detail:\n%s", rendered)
	}
}

// TestSessionReloadMultipleToolTypesRenderDistinct: two different tool
// types must not merge into one unnamed group (the bare-caret collapse).
func TestSessionReloadMultipleToolTypesRenderDistinct(t *testing.T) {
	m := NewChatModel()
	m.width = 120
	m.height = 24
	m.resize(120, 24)

	sessionMsgs := []types.Message{
		{
			UUID: "t1", Role: types.RoleAssistant,
			Content: []types.ContentBlock{
				types.ToolUseBlock{ID: "c1", Name: "bash", Input: map[string]any{"command": "ls"}},
				types.ToolResultBlock{ToolUseID: "c1", Content: "ok"},
			},
			Timestamp: time.Now(),
		},
		{
			UUID: "t2", Role: types.RoleAssistant,
			Content: []types.ContentBlock{
				types.ToolUseBlock{ID: "c2", Name: "grep", Input: map[string]any{"pattern": "todo"}},
				types.ToolResultBlock{ToolUseID: "c2", Content: "match"},
			},
			Timestamp: time.Now(),
		},
	}

	m.SetMessages(sessionMsgs)

	names := map[string]bool{}
	for i := range m.messages {
		if m.messages[i].IsTool {
			names[m.messages[i].ToolDisplayName] = true
		}
	}
	if !names["Shell"] || !names["Search"] {
		t.Fatalf("tool display names after reload = %v, want Shell and Search", names)
	}
}

// TestSessionReloadTextMessageUntouched: plain text messages must not
// gain tool fields.
func TestSessionReloadTextMessageUntouched(t *testing.T) {
	m := NewChatModel()
	m.SetMessages([]types.Message{
		{UUID: "u1", Role: types.RoleUser,
			Content:   []types.ContentBlock{types.TextBlock{Text: "hello"}},
			Timestamp: time.Now()},
	})
	if len(m.messages) != 1 || m.messages[0].IsTool {
		t.Fatalf("text message misrendered as tool: %+v", m.messages[0])
	}
	if m.messages[0].ToolDisplayName != "" {
		t.Fatal("text message gained a ToolDisplayName")
	}
}

func indexTool(messages []ChatMessage) int {
	for i := range messages {
		if messages[i].IsTool {
			return i
		}
	}
	return 0
}
