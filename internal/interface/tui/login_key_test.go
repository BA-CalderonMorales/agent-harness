package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestLoginKeyRemovesClipboardControls(t *testing.T) {
	for _, stored := range []bool{false, true} {
		t.Run(map[bool]string{false: "pasted", true: "stored"}[stored], func(t *testing.T) {
			d := NewLoginDialog()
			d.Open(120, 40, NewStoredCredentials(map[string]string{"openai": " \x00test-key\r\n"}, ""))
			d.providerIdx = 1
			if !stored {
				d.stored = StoredCredentials{}
				d.step = LoginStepAPIKey
				d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" \x00test-key\r\n"), Paste: true})
			}
			d.SetModelsProvider(func(provider, key string) ([]ModelItem, error) {
				if key != "test-key" {
					t.Fatal("probe received an unnormalized credential")
				}
				return nil, nil
			})
			d.Update(tea.KeyMsg{Type: tea.KeyEnter})
			completed, _, _, key, _ := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
			if !completed || key != "test-key" {
				t.Fatal("completion must return the normalized credential")
			}
		})
	}
}

func TestLoginKeyRejectsOnlyControls(t *testing.T) {
	d := NewLoginDialog()
	d.step = LoginStepAPIKey
	d.apiKeyBuf = "\x00\r\n"
	d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if d.step != LoginStepAPIKey || d.errorMsg == "" {
		t.Fatal("empty normalized key must stay on the key step")
	}
}
