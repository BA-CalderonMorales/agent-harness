package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestLoginDialogStoredKeysPerProvider pins the per-provider retention:
// a stored key skips the key step for ITS provider and completes with
// that key; a provider without a stored key requires one (empty submit
// is rejected) — the single-slot store used to 401 both ways.
func TestLoginDialogStoredKeysPerProvider(t *testing.T) {
	d := NewLoginDialog()

	// No stored keys: picking a hosted provider (openai) lands on the
	// key step, and an empty submit is rejected.
	d.Open(80, 24, NewStoredCredentials(nil, ""))
	d.Update(tea.KeyMsg{Type: tea.KeyDown}) // local -> openai
	done, cancelled, _, apiKey, _ := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if done || cancelled || apiKey != "" {
		t.Fatalf("no-key empty submit: done=%v cancelled=%v apiKey=%q, want the key step", done, cancelled, apiKey)
	}

	// A stored openrouter key: picking openrouter skips the key step and
	// completes with the stored key.
	d = NewLoginDialog()
	d.Open(80, 24, NewStoredCredentials(map[string]string{"openrouter": "sk-or-real"}, "sk-or-real"))
	for i := 0; i < 3; i++ { // local -> openai -> anthropic -> openrouter
		d.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	done, cancelled, _, apiKey, _ = d.Update(tea.KeyMsg{Type: tea.KeyEnter}) // complete
	if !done || cancelled {
		t.Fatalf("stored-key flow did not complete: done=%v cancelled=%v", done, cancelled)
	}
	if apiKey != "sk-or-real" {
		t.Fatalf("completed apiKey = %q, want the stored openrouter key", apiKey)
	}
}

// TestProviderPickerFlow pins the provider-switch modal: navigation, a
// completed pick, and cancel.
func TestProviderPickerFlow(t *testing.T) {
	p := NewProviderPicker()
	p.Open(80, 24)
	if !p.IsShowing() {
		t.Fatalf("picker did not open")
	}

	// Navigate down and pick the second provider.
	_, cancelled, _ := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if cancelled {
		t.Fatalf("navigation reported cancelled")
	}
	done, cancelled, provider := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !done || cancelled {
		t.Fatalf("enter: done=%v cancelled=%v, want completed pick", done, cancelled)
	}
	if provider != loginProviders[1] {
		t.Fatalf("picked %q, want %q", provider, loginProviders[1])
	}

	// Cancel path.
	p.Open(80, 24)
	done, cancelled, _ = p.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if done || !cancelled {
		t.Fatalf("esc: done=%v cancelled=%v, want cancel", done, cancelled)
	}
}
