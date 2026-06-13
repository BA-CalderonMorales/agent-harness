package tui

import "testing"

func TestCompletedToolActivityStaysBeforeFollowingOutput(t *testing.T) {
	chat := NewChatModel()

	chat.AddMessage("user", "inspect the repo")
	chat.AddOrUpdateToolMessage("tool-1", "bash", "Shell", "git status", ToolStatusRunning)
	chat.AddOrUpdateToolMessage("tool-1", "bash", "Shell", "git status", ToolStatusSuccess)
	chat.AddMessage("assistant", "Working tree is clean.")

	if len(chat.messages) != 3 {
		t.Fatalf("messages = %d, want 3", len(chat.messages))
	}
	if got := chat.messages[1]; !got.IsTool || got.ToolStatus != ToolStatusSuccess {
		t.Fatalf("middle message = %#v, want completed tool activity", got)
	}
	if got := chat.messages[2]; got.Role != "assistant" || got.Content != "Working tree is clean." {
		t.Fatalf("last message = %#v, want assistant output after tool", got)
	}
}
