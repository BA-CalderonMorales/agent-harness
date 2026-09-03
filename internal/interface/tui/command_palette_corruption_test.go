package tui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/interface/commands"
	tea "github.com/charmbracelet/bubbletea"
)

// TestCommandPaletteSearchDoesNotCorruptList guards the slice-aliasing
// bug where filtering wrote through the backing array shared with the
// command list: searching must never duplicate or drop entries.
func TestCommandPaletteSearchDoesNotCorruptList(t *testing.T) {
	reg := commands.NewSlashRegistry()
	reg.Register("persona", "Switch behavior mode", func(string) (string, error) { return "", nil })
	reg.Register("permissions", "Show permissions", func(string) (string, error) { return "", nil })
	reg.Register("audit", "Show tool activity", func(string) (string, error) { return "", nil })

	p := NewCommandPalette()
	p.SetCommands(reg.GetCommandInfos())
	p.Open(120, 40)

	for _, ch := range "persona" {
		p.Update(teaKeyMsgRune(ch))
	}
	content := p.buildContent()
	if got := strings.Count(content, "/persona"); got != 1 {
		t.Fatalf("searching persona rendered %d /persona rows, want 1", got)
	}

	// The source list must survive the search intact.
	names := make([]string, len(p.commands))
	for i, c := range p.commands {
		names[i] = c.Command
	}
	want := []string{"/audit", "/permissions", "/persona"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("commands after search = %v, want %v (search must not corrupt the list)", names, want)
	}
}

func teaKeyMsgRune(ch rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(string(ch))}
}
