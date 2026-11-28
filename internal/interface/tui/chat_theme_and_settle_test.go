package tui

import (
	"errors"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestApplyThemeInvalidatesMarkdownCaches(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(prevProfile)
		ApplyTheme("default")
	})

	ApplyTheme("default")
	chat := NewChatModel()
	chat.width = 100
	chat.height = 30
	chat.AddMessage("assistant", "# Heading\n\nBody text")
	before := chat.viewport.View()

	markdownRenderersMu.Lock()
	hadRenderer := len(markdownRenderers) > 0
	markdownRenderersMu.Unlock()
	if !hadRenderer && !isTermux {
		t.Fatal("expected markdown renderer to be cached before theme switch")
	}
	renderCacheStore.mu.Lock()
	hadRenderEntry := len(renderCacheStore.ents) > 0
	renderCacheStore.mu.Unlock()
	if !hadRenderEntry {
		t.Fatal("expected rendered markdown to be cached before theme switch")
	}
	groupCacheStore.mu.Lock()
	hadGroupEntry := len(groupCacheStore.ents) > 0
	groupCacheStore.mu.Unlock()
	if !hadGroupEntry {
		t.Fatal("expected rendered turn group to be cached before theme switch")
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
	renderCacheStore.mu.Lock()
	remainingRenders := len(renderCacheStore.ents)
	renderCacheStore.mu.Unlock()
	if remainingRenders != 0 {
		t.Fatalf("renderCacheStore still had %d stale entries after ApplyTheme", remainingRenders)
	}
	groupCacheStore.mu.Lock()
	remainingGroups := len(groupCacheStore.ents)
	groupCacheStore.mu.Unlock()
	if remainingGroups != 0 {
		t.Fatalf("groupCacheStore still had %d stale entries after ApplyTheme", remainingGroups)
	}

	chat.refreshViewport()
	after := chat.viewport.View()
	if before == after {
		t.Fatal("rendered chat viewport did not change after switching theme from default to nord")
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
