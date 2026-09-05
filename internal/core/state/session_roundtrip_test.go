package state

import (
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// TestSessionRoundTripPreservesInterleaving is the reproducible-sessions
// promise as a contract: a turn with a tool interruption — text, tool
// call, tool result, more text — must survive save → load with roles,
// block order, and tool-use identity intact. Everything downstream
// (resume, export, the TUI's segmented rendering) stands on this.
func TestSessionRoundTripPreservesInterleaving(t *testing.T) {
	sessionsDir := t.TempDir()
	t.Setenv("AGENT_HARNESS_SESSION_DIR", sessionsDir)

	manager, err := NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	session := manager.CreateSession("test-model")
	session.ID = "roundtrip-session"
	session.AddMessage(types.Message{
		Role:    types.RoleUser,
		Content: []types.ContentBlock{types.TextBlock{Text: "weather?"}},
	})
	session.AddMessage(types.Message{
		Role: types.RoleAssistant,
		Content: []types.ContentBlock{
			types.TextBlock{Text: "On it — pulling the forecast."},
			types.ToolUseBlock{ID: "call-1", Name: "web_fetch", Input: map[string]any{"url": "https://wttr.in/Omaha"}},
			types.TextBlock{Text: "Got the data — tomorrow is a scorcher."},
		},
	})
	session.AddMessage(types.Message{
		Role:    types.RoleUser,
		Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: "call-1", Content: `{"temp": 96}`}},
	})

	if _, err := manager.SaveCurrent(); err != nil {
		t.Fatalf("SaveCurrent() error = %v", err)
	}

	// A fresh manager reads the same directory: resume, not memory.
	reloaded, err := NewSessionManager()
	if err != nil {
		t.Fatalf("reloaded manager error = %v", err)
	}
	loaded, err := reloaded.LoadSession("roundtrip-session")
	if err != nil {
		t.Fatalf("LoadSession() error = %v", err)
	}

	if len(loaded.Messages) != 3 {
		t.Fatalf("loaded %d messages, want 3", len(loaded.Messages))
	}

	assistant := loaded.Messages[1]
	if assistant.Role != types.RoleAssistant {
		t.Fatalf("assistant role = %q", assistant.Role)
	}
	if len(assistant.Content) != 3 {
		t.Fatalf("assistant blocks = %d, want 3 (text, tool_use, text) — the interleaving is the contract", len(assistant.Content))
	}
	if text, ok := assistant.Content[0].(types.TextBlock); !ok || text.Text != "On it — pulling the forecast." {
		t.Fatalf("block 0 = %#v, want the opening prose", assistant.Content[0])
	}
	if use, ok := assistant.Content[1].(types.ToolUseBlock); !ok || use.ID != "call-1" || use.Name != "web_fetch" {
		t.Fatalf("block 1 = %#v, want the web_fetch tool use with its ID", assistant.Content[1])
	}
	if result, ok := loaded.Messages[2].Content[0].(types.ToolResultBlock); !ok || result.ToolUseID != "call-1" {
		t.Fatalf("result block = %#v, want the tool result bound to call-1", loaded.Messages[2].Content[0])
	}
}
