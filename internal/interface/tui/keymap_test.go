package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// The keymap is data, so it can be listed and checked.

func TestEveryBindingIsDescribed(t *testing.T) {
	for _, group := range [][]keyBinding{globalKeyBindings, viewKeyBindings} {
		for _, binding := range group {
			if len(binding.Keys) == 0 {
				t.Errorf("binding %q has no keys", binding.Action)
			}
			if binding.Action == "" {
				t.Errorf("binding %v has no action name", binding.Keys)
			}
			if binding.Description == "" {
				t.Errorf("binding %q has no description", binding.Action)
			}
			if binding.Run == nil {
				t.Errorf("binding %q has no action", binding.Action)
			}
		}
	}
}

func TestBindingRowsAreListed(t *testing.T) {
	rows := BindingRows()
	if len(rows) == 0 {
		t.Fatal("BindingRows() is empty")
	}
	joined := strings.Join(rows, "\n")
	for _, want := range []string{"command palette", "reasoning effort", "home", "settings"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("binding rows are missing %q:\n%s", want, joined)
		}
	}
}

func TestViewBindingSwitchesViews(t *testing.T) {
	app := NewApp()
	app.width, app.height = 80, 24
	app.activeView = viewHome

	_, handled, claimed := app.handleViewBinding(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	if !claimed {
		t.Fatal("handleViewBinding(3) did not claim the key")
	}
	if !handled {
		t.Fatal("handleViewBinding(3) did not handle the key")
	}
	if app.activeView != viewSessions {
		t.Fatalf("active view = %v, want sessions", app.activeView)
	}
}

func TestGlobalBindingGuardsInsertMode(t *testing.T) {
	app := NewApp()
	app.width, app.height = 80, 24
	app.mode = ModeInsert

	// ':' belongs to the composer in insert mode, so the binding must
	// not claim it.
	if _, _, claimed := app.handleGlobalBinding(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")}); claimed {
		t.Fatal("the ':' binding fired in insert mode")
	}

	app.mode = ModeNormal
	if _, _, claimed := app.handleGlobalBinding(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")}); !claimed {
		t.Fatal("the ':' binding did not fire in navigate mode")
	}
}
