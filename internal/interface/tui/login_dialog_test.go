package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// newTestLoginDialog wires a fake models provider and opens the dialog.
func newTestLoginDialog(t *testing.T, models LoginModelsProvider) *LoginDialogModel {
	t.Helper()
	d := NewLoginDialog()
	d.SetModelsProvider(models)
	d.Open(120, 40, StoredCredentials{})
	return &d
}

// advanceToModelStep drives provider (local = no key step) -> model step.
func advanceToModelStep(d *LoginDialogModel) {
	d.Update(tea.KeyMsg{Type: tea.KeyEnter})
}

func TestLoginDialogModelStepShowsLiveModels(t *testing.T) {
	live := []ModelItem{
		{ID: "demo-1.0", Name: "demo-1.0", Provider: "local", IsDefault: true},
		{ID: "other-model", Name: "other-model", Provider: "local"},
	}
	d := newTestLoginDialog(t, func(provider, apiKey string) ([]ModelItem, error) {
		if provider != "local" {
			t.Fatalf("provider = %q, want local", provider)
		}
		return live, nil
	})
	advanceToModelStep(d)

	if d.step != LoginStepModel {
		t.Fatalf("step = %v, want LoginStepModel", d.step)
	}
	content := d.picker.viewport.View()
	if !strings.Contains(content, "demo-1.0") {
		t.Fatalf("model step does not show the live model list:\n%s", content)
	}
	if !strings.Contains(content, "[default]") {
		t.Fatalf("model step does not pin the provider default:\n%s", content)
	}
	if d.probeErr != "" {
		t.Fatalf("probeErr = %q, want empty for a live list", d.probeErr)
	}
}

func TestLoginDialogModelStepFailingProbeShowsErrorNotSilentDefault(t *testing.T) {
	probeErr := errors.New("connection refused")
	catalog := []ModelItem{{ID: "catalog-model", Name: "catalog-model", Provider: "openai"}}
	var sawKey string
	d := newTestLoginDialog(t, func(provider, apiKey string) ([]ModelItem, error) {
		sawKey = apiKey
		return catalog, probeErr
	})

	// openai goes through the API key step first.
	d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) // openai
	d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	for _, ch := range "sk-test-123" {
		d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(string(ch))})
	}
	d.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if sawKey != "sk-test-123" {
		t.Fatalf("models provider got key %q, want the typed key", sawKey)
	}
	if d.probeErr == "" {
		t.Fatal("failing probe must surface an error, not a silent default")
	}
	if !strings.Contains(d.probeErr, "connection refused") {
		t.Fatalf("probeErr = %q, want the probe error text", d.probeErr)
	}
	view := d.View()
	if !strings.Contains(view, "catalog-model") {
		t.Fatalf("static catalog fallback not shown:\n%s", view)
	}
	if !strings.Contains(view, "Could not reach endpoint") {
		t.Fatalf("view does not surface the probe error:\n%s", view)
	}
}

func TestLoginDialogModelStepEnterCompletesWithSelection(t *testing.T) {
	live := []ModelItem{{ID: "pick-me", Name: "pick-me", Provider: "local"}}
	d := newTestLoginDialog(t, func(provider, apiKey string) ([]ModelItem, error) {
		return live, nil
	})
	advanceToModelStep(d)

	closed, cancelled, provider, apiKey, model := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !closed || cancelled {
		t.Fatalf("closed=%v cancelled=%v, want closed with no cancel", closed, cancelled)
	}
	if provider != "local" || model != "pick-me" || apiKey != "" {
		t.Fatalf("completion = (%q, %q, %q), want (local, pick-me, empty)", provider, apiKey, model)
	}
}

func TestLoginDialogModelStepEnterWithoutSelectionFallsBackToDefault(t *testing.T) {
	// No models at all: Enter must still finish with the empty model so
	// completeLogin's empty->default fallback applies.
	d := newTestLoginDialog(t, func(provider, apiKey string) ([]ModelItem, error) {
		return nil, nil
	})
	advanceToModelStep(d)

	closed, cancelled, _, _, model := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !closed || cancelled || model != "" {
		t.Fatalf("closed=%v cancelled=%v model=%q, want closed with empty model", closed, cancelled, model)
	}
}

func TestLoginDialogProviderStepShowsBlurbs(t *testing.T) {
	d := newTestLoginDialog(t, nil)
	view := d.View()
	for provider, blurb := range providerBlurbs {
		if !strings.Contains(view, blurb) {
			t.Fatalf("provider step missing blurb for %q", provider)
		}
	}
}

func TestLoginDialogModelStepEscCancelsAndFilters(t *testing.T) {
	live := []ModelItem{
		{ID: "alpha-model", Name: "alpha-model", Provider: "local"},
		{ID: "beta-model", Name: "beta-model", Provider: "local"},
	}
	d := newTestLoginDialog(t, func(provider, apiKey string) ([]ModelItem, error) {
		return live, nil
	})
	advanceToModelStep(d)

	// Typing filters the picker (one rune per key, like real input).
	for _, ch := range "beta" {
		d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(string(ch))})
	}
	if !strings.Contains(d.picker.viewport.View(), "beta-model") {
		t.Fatal("filter did not narrow the model list")
	}
	if strings.Contains(d.picker.viewport.View(), "alpha-model") {
		t.Fatal("filter did not exclude non-matching models")
	}

	closed, cancelled, _, _, _ := d.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if closed || !cancelled {
		t.Fatalf("Esc: closed=%v cancelled=%v, want cancelled wizard", closed, cancelled)
	}
}
