package tui

import (
	"errors"
	"testing"
	"time"
)

func TestApplyThemeInvalidatesMarkdownCaches(t *testing.T) {
	t.Cleanup(func() { ApplyTheme("default") })

	ApplyTheme("default")
	_ = renderMarkdown("# Heading", 80)

	markdownRenderersMu.Lock()
	hadRenderer := len(markdownRenderers) > 0
	markdownRenderersMu.Unlock()
	if !hadRenderer && !isTermux {
		t.Fatal("expected markdown renderer to be cached before theme switch")
	}
	if _, ok := renderCacheStore.get("# Heading", 80); !ok {
		t.Fatal("expected rendered markdown to be cached before theme switch")
	}

	if !ApplyTheme("nord") {
		t.Fatal("ApplyTheme(nord) = false")
	}
	markdownRenderersMu.Lock()
	remainingRenderers := len(markdownRenderers)
	markdownRenderersMu.Unlock()
	if remainingRenderers != 0 {
		t.Fatalf("markdownRenderers still had %d stale entries after ApplyTheme", remainingRenderers)
	}
	if _, ok := renderCacheStore.get("# Heading", 80); ok {
		t.Fatal("stale rendered markdown survived ApplyTheme")
	}
}

func TestCancelAndErrorSettleAllParallelRunningTools(t *testing.T) {
	for _, tc := range []struct {
		name string
		msg  any
	}{
		{"cancel", AgentCancelMsg{}},
		{"error", AgentErrorMsg{Error: errors.New("provider reset"), Timestamp: time.Now()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chat := NewChatModel()
			chat.width = 100
			chat.height = 30

			model, _ := chat.Update(AgentStartMsg{Timestamp: time.Now()})
			chat = model.(ChatModel)
			model, _ = chat.Update(AgentToolStartMsg{ToolID: "t1", ToolName: "read", DisplayName: "Read", ActivityDesc: "a.go"})
			chat = model.(ChatModel)
			model, _ = chat.Update(AgentToolStartMsg{ToolID: "t2", ToolName: "grep", DisplayName: "Search", ActivityDesc: "foo"})
			chat = model.(ChatModel)
			model, _ = chat.Update(AgentToolStartMsg{ToolID: "t3", ToolName: "glob", DisplayName: "Find Files", ActivityDesc: "*.go"})
			chat = model.(ChatModel)

			model, _ = chat.Update(tc.msg)
			chat = model.(ChatModel)

			settled := 0
			for _, m := range chat.messages {
				if !m.IsTool {
					continue
				}
				if m.ToolStatus != ToolStatusError {
					t.Errorf("tool %s stayed in status %q after %s, want error", m.ID, m.ToolStatus, tc.name)
				}
				settled++
			}
			if settled != 3 {
				t.Fatalf("settled %d tool messages, want 3", settled)
			}
		})
	}
}
