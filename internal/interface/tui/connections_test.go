package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func loginProviderIndex(provider string) int {
	for i, p := range loginProviders {
		if p == provider {
			return i
		}
	}
	panic("unknown provider: " + provider)
}

func TestConnectionsSeparateLocalAndHosted(t *testing.T) {
	d := NewLoginDialog()
	d.Open(120, 50, StoredCredentials{})
	view := ansi.Strip(d.View())
	for _, label := range []string{"On this computer", "Hosted inference", "Lemonade", "FastFlowLM (direct)", "Custom local server"} {
		if !strings.Contains(view, label) {
			t.Fatalf("missing %q:\n%s", label, view)
		}
	}
	if strings.Index(view, "Lemonade") > strings.Index(view, "Hosted inference") {
		t.Fatal("Lemonade belongs in the local group")
	}
	// Every row remains reachable on a short pane; headings do not consume a key.
	for i, provider := range loginProviders {
		d.providerIdx = i
		d.width, d.height = 40, 15
		if !strings.Contains(ansi.Strip(d.View()), connectionLabel(provider)) {
			t.Fatalf("selected %s is hidden", provider)
		}
	}
}

func TestLemonadeLoginRefreshAndRecovery(t *testing.T) {
	d := NewLoginDialog()
	d.Open(120, 40, StoredCredentials{})
	d.providerIdx = loginProviderIndex("lemonade")
	ready := false
	d.SetModelsProvider(func(provider, key string) ([]ModelItem, error) {
		if provider != "lemonade" || key != "" {
			t.Fatal("local discovery should not need a key")
		}
		if !ready {
			return nil, errors.New("connection refused")
		}
		return []ModelItem{{ID: "qwen3.5-2b-FLM", Name: "Qwen 3.5 2B"}, {ID: "qwen3.5-4b-FLM", Name: "Qwen 3.5 4B", IsDefault: true}}, nil
	})
	d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if d.step != LoginStepModel || d.probeErr == "" {
		t.Fatal("must show connection error without a key step")
	}
	done, _, _, _, _ := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if done {
		t.Fatal("unreachable Lemonade must not claim a successful login")
	}
	ready = true
	d.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	for _, r := range "qwen" {
		d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	done, canceled, provider, _, model := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !done || canceled || provider != "lemonade" || model != "qwen3.5-4b-FLM" {
		t.Fatal("retry did not select the live model")
	}
	ready = false
	d.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	if len(d.picker.filtered) != 0 {
		t.Fatal("failed refresh retained a stale model list")
	}
	d.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	if d.step != LoginStepProvider {
		t.Fatal("back must allow a different connection")
	}
}

func TestModelNamesAreSearchText(t *testing.T) {
	for _, name := range []string{"qwen", "gemma", "deepseek", "jamba"} {
		picker := NewModelPicker()
		picker.SetModels([]ModelItem{{ID: name, Name: name}, {ID: "other", Name: "other"}})
		picker.Open(100, 30)
		for _, r := range name {
			closed, _ := picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			if closed {
				t.Fatalf("typing %s closed the picker", name)
			}
		}
		if picker.searchQuery != name || len(picker.filtered) != 1 || picker.filtered[0].ID != name {
			t.Fatalf("model name %s did not filter correctly", name)
		}
	}
}
