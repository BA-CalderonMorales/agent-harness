package tui

import (
	"path/filepath"
	"strings"
	"testing"
)

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

func TestRepeatedToolActivityRendersAsCompactScanRows(t *testing.T) {
	chat := NewChatModel()
	chat.width = 72
	chat.height = 20
	longPath := filepath.Join(t.TempDir(), "nested", "sample-project", "README.md")

	chat.AddOrUpdateToolMessage("tool-1", "bash", "Ran", "rtk git status --short --branch", ToolStatusSuccess)
	chat.AddOrUpdateToolMessage("tool-2", "bash", "Ran", "rtk go test ./internal/interface/tui", ToolStatusRunning)
	chat.AddOrUpdateToolMessage("tool-3", "read", "Read", longPath, ToolStatusError)

	view := chat.viewport.View()
	for _, want := range []string{"Ran", "Read", "rtk git status", "rtk go test", "README.md"} {
		if !strings.Contains(view, want) {
			t.Fatalf("tool transcript missing %q\n%s", want, view)
		}
	}
	if strings.Count(view, "\n\n\n") > 0 {
		t.Fatalf("tool rows should not render with large vertical gaps\n%s", view)
	}
	if strings.Contains(view, longPath) {
		t.Fatalf("long paths should be truncated in compact tool rows\n%s", view)
	}
}
