package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func newLoginLinkApp(t *testing.T, provider string, width int) *App {
	t.Helper()
	a := NewApp()
	a.Update(tea.WindowSizeMsg{Width: width, Height: 36})
	a.OpenLoginDialog(StoredCredentials{})
	for i, p := range loginProviders {
		if p == provider {
			a.loginDialog.providerIdx = i
			break
		}
	}
	a.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if a.loginDialog.step != LoginStepAPIKey {
		t.Fatal("provider did not reach API-key step")
	}
	a.loginDialog.apiKeyBuf = "test-secret"
	return a
}

func TestLoginLinkKeyboardActions(t *testing.T) {
	for provider, steps := range providerKeySteps {
		for _, key := range []tea.KeyType{tea.KeyCtrlO, tea.KeyCtrlY} {
			t.Run(provider+"/"+key.String(), func(t *testing.T) {
				a := newLoginLinkApp(t, provider, 100)
				_, cmd := a.Update(tea.KeyMsg{Type: key})
				if cmd == nil {
					t.Fatal("link shortcut did not produce an action")
				}
				request, ok := cmd().(loginLinkRequestMsg)
				if !ok || request.url != steps.URL || request.copyURL != (key == tea.KeyCtrlY) {
					t.Fatalf("shortcut request = %+v", request)
				}
				_, action := a.Update(request)
				if action == nil {
					t.Fatal("link request did not schedule its command")
				}
				if a.loginDialog.apiKeyBuf != "test-secret" || a.loginDialog.step != LoginStepAPIKey {
					t.Fatal("link shortcut changed the key or advanced the wizard")
				}
			})
		}
	}
}

func TestLoginLinkMouseTargetsRenderedURL(t *testing.T) {
	for provider, steps := range providerKeySteps {
		for _, width := range []int{40, 80, 120} {
			t.Run(fmt.Sprintf("%s/%d", provider, width), func(t *testing.T) {
				a := newLoginLinkApp(t, provider, width)
				view := a.View()
				if !strings.Contains(view, ansi.SetHyperlink(steps.URL)) {
					t.Fatal("rendered URL has no native hyperlink")
				}
				found := false
				for y, line := range strings.Split(ansi.Strip(view), "\n") {
					if x := strings.Index(line, "https://"); x >= 0 {
						if a.loginDialog.linkAt(0, y, steps.URL) || a.loginDialog.linkAt(width-1, y, steps.URL) {
							t.Fatal("URL hit target extends into the panel margin")
						}
						click := tea.MouseMsg{X: ansi.StringWidth(line[:x]), Y: y,
							Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
						_, cmd := a.Update(click)
						if cmd == nil {
							t.Fatal("click on URL did not produce an action")
						}
						request, ok := cmd().(loginLinkRequestMsg)
						if !ok || request.url != steps.URL || request.copyURL {
							t.Fatalf("click request = %+v", request)
						}
						click.Action = tea.MouseActionRelease
						if _, cmd := a.Update(click); cmd != nil {
							t.Fatal("mouse release opened a second browser")
						}
						found = true
					}
				}
				if !found {
					t.Fatal("URL was not visible")
				}
				if _, cmd := a.Update(tea.MouseMsg{X: 0, Y: 0,
					Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}); cmd != nil {
					t.Fatal("click outside the link opened a browser")
				}
			})
		}
	}
}

func TestLoginLinkActionsReportInsideModal(t *testing.T) {
	for _, copyURL := range []bool{false, true} {
		for _, succeed := range []bool{false, true} {
			t.Run(fmt.Sprintf("copy=%v/succeed=%v", copyURL, succeed), func(t *testing.T) {
				a := newLoginLinkApp(t, "openrouter", 100)
				url := providerKeySteps["openrouter"].URL
				opened, copied := "", ""
				cmd := loginLinkActionCmd(loginLinkRequestMsg{url: url, copyURL: copyURL},
					func(u string) error {
						opened = u
						if !succeed {
							return errors.New("browser unavailable")
						}
						return nil
					}, func(u string) bool { copied = u; return succeed })
				result := cmd().(loginLinkResultMsg)
				if (copyURL && (copied != url || opened != "")) || (!copyURL && (opened != url || copied != "")) {
					t.Fatalf("wrong destination or transport: opened=%q copied=%q", opened, copied)
				}
				a.Update(result)
				plain := ansi.Strip(a.View())
				if result.notice == "" || !strings.Contains(plain, result.notice) {
					t.Fatalf("action feedback missing from modal: %q", result.notice)
				}
				if strings.Contains(plain, "test-secret") || a.loginDialog.apiKeyBuf != "test-secret" {
					t.Fatal("link action exposed or changed the API key")
				}
				if succeed == strings.Contains(result.notice, "Could not") {
					t.Fatalf("incorrect success/error notice: %q", result.notice)
				}
			})
		}
	}
}

func TestLoginLinkShortcutsDoNotEatPastesOrOtherSteps(t *testing.T) {
	a := newLoginLinkApp(t, "openai", 100)
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ctrl+y"), Paste: true})
	if a.loginDialog.apiKeyBuf != "test-secretctrl+y" {
		t.Fatal("link shortcut intercepted a pasted credential")
	}
	a.loginDialog.Close()
	a.OpenLoginDialog(StoredCredentials{})
	if _, cmd := a.Update(tea.KeyMsg{Type: tea.KeyCtrlY}); cmd != nil {
		t.Fatal("URL action ran outside the API-key step")
	}
	a.Update(loginLinkResultMsg{url: providerKeySteps["openai"].URL, notice: "old result"})
	if a.loginDialog.linkNotice != "" {
		t.Fatal("old action result changed a newly opened wizard")
	}
}
