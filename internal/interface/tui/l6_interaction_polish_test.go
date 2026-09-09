package tui

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestL6ToolRowKeepsUnicodeAndWidth(t *testing.T) {
	model := NewChatModel()
	command := "echo " + strings.Repeat("😀", 30)
	row := model.formatToolContentAt(60, "Shell", command, "#abcd", ToolStatusSuccess, time.Date(2026, 9, 7, 12, 34, 56, 0, time.UTC), 2*time.Second)
	if !utf8.ValidString(row) {
		t.Fatalf("tool row is not valid UTF-8: %q", row)
	}
	if got := lipgloss.Width(row); got > 60 {
		t.Fatalf("tool row width = %d, want <= 60: %q", got, row)
	}
	if strings.Contains(row, "�") {
		t.Fatalf("long Unicode detail was cut through a rune: %q", row)
	}
}

func TestR6ToolTruncationKeepsGraphemesANSIAndDisplayWidth(t *testing.T) {
	model := NewChatModel()
	for _, command := range []string{
		"echo " + strings.Repeat("界", 40),
		"echo " + strings.Repeat("🏳️‍🌈", 20),
		"echo \x1b[31m" + strings.Repeat("界", 20) + "\x1b[0m",
	} {
		row := model.formatToolContentAt(48, "Shell", command, "#abcd", ToolStatusSuccess, time.Now(), time.Second)
		if !utf8.ValidString(row) {
			t.Fatalf("truncated row is not valid UTF-8: %q", row)
		}
		if got := ansi.StringWidth(row); got > 48 {
			t.Fatalf("truncated row width = %d, want <= 48: %q", got, row)
		}
	}
}

func TestR6ModeLineTransitionsAtMobileWidths(t *testing.T) {
	model := NewChatModel()
	model.SetModeLabel("navigate")
	model.SetModel("old-model")
	model.SetProvider("old-provider")
	model.SetPersona("old-persona")
	model.SetAgentMode("manual")
	model.SetEffort("low")
	for _, width := range []int{40, 48, 60, 80} {
		model.width = width
		model.SetModeLabel("navigate")
		model.SetModel("old-model")
		model.SetProvider("old-provider")
		model.SetPersona("old-persona")
		model.SetAgentMode("manual")
		model.SetEffort("low")
		before := ansi.Strip(model.renderModeLine())
		model.SetModel("new-model")
		model.SetProvider("new-provider")
		model.SetPersona("new-persona")
		model.SetAgentMode("auto")
		model.SetEffort("high")
		after := model.renderModeLine()
		if got := ansi.StringWidth(after); got > width {
			t.Fatalf("width %d: mode line width = %d, want <= %d: %q", width, got, width, after)
		}
		if width >= 80 && (strings.Contains(before, "new-model") || !strings.Contains(ansi.Strip(after), "new-model")) {
			t.Fatalf("width %d: state transition not reflected in next frame: before=%q after=%q", width, before, after)
		}
		model.SetModeLabel("typing")
		if line := ansi.Strip(model.renderModeLine()); !strings.Contains(line, "typing") {
			t.Fatalf("width %d: mode transition missing from next frame: %q", width, line)
		}
		model.SetModeLabel("navigate")
	}
}

func TestR6WorkingStatusReservesComposerSpace(t *testing.T) {
	for _, width := range []int{40, 48, 60, 80} {
		model := NewChatModel()
		model.width = width
		model.height = 20
		model.SetInput("ready")
		model.SetThinking(true, "Thinking...")
		view := model.View()
		lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
		if len(lines) > model.height {
			t.Fatalf("width %d: active view has %d rows, want <= %d", width, len(lines), model.height)
		}
		if !strings.Contains(view, "Working") || !strings.Contains(view, "ready") || !strings.Contains(view, "effort") {
			t.Fatalf("width %d: working/status/composer hierarchy missing:\n%s", width, view)
		}
	}
}

func TestL6ModeLineDoesNotClipMetadataMidWord(t *testing.T) {
	model := NewChatModel()
	model.modeLabel = "navigate"
	model.persona = "very-long-persona-name"
	model.model = "provider/very-long-model-name"
	model.provider = "very-long-provider-name"
	model.effort = "high"
	for _, width := range []int{60, 80, 120, 160} {
		model.width = width
		line := model.renderModeLine()
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("width %d: mode line width = %d, want <= %d: %q", width, got, width, line)
		}
		if width == 60 && strings.Contains(line, "very-long-provider") {
			t.Fatalf("width %d: narrow mode line retained low-priority metadata: %q", width, line)
		}
	}
}
