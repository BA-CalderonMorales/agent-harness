package tui

import (
	"reflect"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/interface/commands"
)

// TestCommandPaletteMatchesRegistry is the single-source-of-truth
// invariant: the palette's command list must be exactly the registry's
// GetCommandInfos names. A hardcoded seed, a duplicate registration, or
// a phantom command all fail here at CI time instead of surfacing to
// users as a palette entry that does nothing.
func TestCommandPaletteMatchesRegistry(t *testing.T) {
	reg := commands.NewSlashRegistry()
	reg.Register("help", "Show help", func(string) (string, error) { return "", nil })
	reg.Register("persona", "Switch behavior mode", func(string) (string, error) { return "", nil })
	reg.Register("audit", "Show tool activity", func(string) (string, error) { return "", nil })

	p := NewCommandPalette()
	p.SetCommands(reg.GetCommandInfos())

	got := make([]string, len(p.commands))
	for i, c := range p.commands {
		got[i] = c.Command
	}
	want := []string{"/audit", "/help", "/persona"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("palette command names = %v, want registry names %v", got, want)
	}
}

// TestCommandPaletteSetCommandsReplacesSeed ensures the palette is never
// seeded with a hardcoded list: a fresh palette is empty until the app
// feeds it the registry, and SetCommands replaces (not appends).
func TestCommandPaletteSetCommandsReplacesSeed(t *testing.T) {
	p := NewCommandPalette()
	if len(p.commands) != 0 {
		t.Fatalf("NewCommandPalette seed has %d commands, want 0 (registry is the only source)", len(p.commands))
	}

	reg := commands.NewSlashRegistry()
	reg.Register("one", "One", func(string) (string, error) { return "", nil })
	p.SetCommands(reg.GetCommandInfos())

	second := commands.NewSlashRegistry()
	second.Register("two", "Two", func(string) (string, error) { return "", nil })
	p.SetCommands(second.GetCommandInfos()) // second feed replaces

	if len(p.commands) != 1 || p.commands[0].Command != "/two" {
		t.Fatalf("palette commands after second SetCommands = %v, want only /two", p.commands)
	}
}
