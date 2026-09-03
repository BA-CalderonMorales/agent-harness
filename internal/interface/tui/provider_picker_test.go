package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestLoginDialogRetainsStoredKey pins the retention UX: when a stored
// key hint is present, finishing the key step with an empty buffer keeps
// the existing secret (empty apiKey is returned); without a hint the
// dialog refuses to continue.
func TestLoginDialogRetainsStoredKey(t *testing.T) {
	d := NewLoginDialog()

	// No stored key: empty submit is rejected.
	d.Open(80, 24, "")
	done, cancelled, _, apiKey, _ := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if done || cancelled || apiKey != "" {
		t.Fatalf("no-hint empty submit: done=%v cancelled=%v apiKey=%q, want rejection", done, cancelled, apiKey)
	}

	// Stored key hint: empty submit advances (model step), then the
	// second Enter completes the wizard with an empty key — the stored
	// secret is retained.
	d.Open(80, 24, "sk-or-…7f75")
	done, cancelled, _, apiKey, _ = d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cancelled || done || apiKey != "" {
		t.Fatalf("retain key-step submit: done=%v cancelled=%v apiKey=%q, want advance to model step", done, cancelled, apiKey)
	}
	done, cancelled, _, apiKey, _ = d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !done || cancelled || apiKey != "" {
		t.Fatalf("retain flow did not complete: done=%v cancelled=%v apiKey=%q, want empty retained key", done, cancelled, apiKey)
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
