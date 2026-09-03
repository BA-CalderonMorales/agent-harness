package tui

import (
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/interface/commands"
	tea "github.com/charmbracelet/bubbletea"
)

// TestColonOpensPaletteFromNormalMode guards the k9s-style ':' binding:
// from a normal-mode view the key opens the command palette, and in
// insert mode it types into the composer untouched.
func TestColonOpensPaletteFromNormalMode(t *testing.T) {
	app := NewApp()
	app.width = 80
	app.height = 24

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")})
	a := model.(*App)
	if cmd == nil {
		t.Fatal("expected openCommandPaletteMsg command")
	}
	if msg := cmd(); msg != (openCommandPaletteMsg{}) {
		t.Fatalf("command = %T, want openCommandPaletteMsg", msg)
	}
	model, _ = a.Update(openCommandPaletteMsg{})
	a = model.(*App)
	if !a.commandPalette.IsShowing() {
		t.Fatal("':' in normal mode did not open the command palette")
	}

	// Insert mode: ':' must reach the composer, not open the palette.
	app2 := NewApp()
	app2.width = 80
	app2.height = 24
	app2.mode = ModeInsert
	app2.chatModel.Focus()
	app2.activeView = viewChat
	model, _ = app2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")})
	a2 := model.(*App)
	if a2.commandPalette.IsShowing() {
		t.Fatal("':' in insert mode opened the palette")
	}
	if got := a2.chatModel.GetInput(); got != ":" {
		t.Fatalf("composer input = %q, want ':' (the key must type)", got)
	}
}

// TestPaletteCategoryHeadersUnique asserts the list never renders the
// same category header twice in a row - the dedupe the owner's M2
// symptom demanded (repeated headers after searching).
func TestPaletteCategoryHeadersUnique(t *testing.T) {
	reg := commands.NewSlashRegistry()
	for _, name := range []string{
		"help", "status", "clear", "compact", "session", "reset", "quit",
		"model", "current-model", "cost", "export", "diff",
		"branch", "pr", "agents", "skills", "plan", "memory", "init",
		"permissions", "config", "login", "logout", "settings",
		"persona", "audit", "steer", "provider", "models", "workspace",
		"version", "effort", "commit",
	} {
		reg.Register(name, "desc", func(string) (string, error) { return "", nil })
	}

	p := NewCommandPalette()
	p.SetCommands(reg.GetCommandInfos())
	p.Open(120, 40)

	categories := map[string]bool{
		"Session": true, "Model": true, "Output": true, "Git": true,
		"Tools": true, "Settings": true, "System": true,
	}
	prev := ""
	headers := 0
	for _, line := range strings.Split(p.buildContent(), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if trimmed == prev {
			t.Fatalf("repeated adjacent line: %q", trimmed)
		}
		prev = trimmed
		if categories[trimmed] {
			headers++
		}
	}
	if headers == 0 {
		t.Fatal("expected category headers in the list")
	}
}

// TestPaletteFooterFitsPanel asserts the footer hint never exceeds the
// palette panel's inner width at any terminal width, so it cannot clip
// or wrap (owner symptom M4).
func TestPaletteFooterFitsPanel(t *testing.T) {
	for _, termWidth := range []int{30, 40, 50, 60, 80, 120} {
		p := NewCommandPalette()
		p.Open(termWidth, 40)
		inner := p.viewport.Width - 4
		for _, canScroll := range []bool{false, true} {
			hint := paletteFooterHint(p.viewport.Width, canScroll)
			if w := lipglossWidth(hint); w > inner {
				t.Fatalf("term width %d (panel %d): footer width %d > inner %d (%q)",
					termWidth, p.viewport.Width, w, inner, hint)
			}
		}
	}
}

func lipglossWidth(s string) int {
	return len(strings.TrimSuffix(s, "\n"))
}
