package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestStatusbarBadgeReflectsReadiness: the badge must tell the truth -
// a misconfigured or unavailable probe renders the dead end WITH its
// handle ((l: login)), and a ready probe renders [ready].
func TestStatusbarBadgeReflectsReadiness(t *testing.T) {
	app := NewApp()
	app.width = 120
	app.height = 40
	app.chatModel.SetModel("demo-1.0") // a model alone must not claim ready

	for _, tc := range []struct {
		readiness int
		want      string
	}{
		{1, "[ready]"},
		{4, "[! setup required] (l: login)"},
		{3, "[! not connected] (l: login)"},
		{0, "[checking"},
	} {
		app.providerReadiness = tc.readiness
		bar := app.renderStatusBar()
		if !strings.Contains(bar, tc.want) {
			t.Fatalf("readiness %d: statusbar %q missing %q", tc.readiness, bar, tc.want)
		}
	}
}

// TestStatusbarReadyWithoutHandle: the handle only advertises on
// broken states, never on a working one.
func TestStatusbarReadyWithoutHandle(t *testing.T) {
	app := NewApp()
	app.width = 120
	app.height = 40
	app.chatModel.SetModel("demo-1.0")
	app.providerReadiness = 1
	bar := app.renderStatusBar()
	if strings.Contains(bar, "(l: login)") {
		t.Fatalf("ready statusbar advertises the login handle: %q", bar)
	}
	if !strings.Contains(bar, "[ready]") {
		t.Fatalf("ready statusbar missing [ready]: %q", bar)
	}
}

// TestHomeSetupBannerHasHandle: the banner renders when the probe
// reports misconfigured and carries the (l: login) affordance.
func TestHomeSetupBannerHasHandle(t *testing.T) {
	home := NewHomeModel()
	home.width = 120
	home.height = 40
	home.SetSetupRequired(true)
	view := home.View()
	if !strings.Contains(view, "Setup Required") {
		t.Fatalf("banner missing when setupRequired:\n%s", view)
	}
	if !strings.Contains(view, "Press l to log in") {
		t.Fatalf("banner missing the l affordance:\n%s", view)
	}

	home2 := NewHomeModel()
	home2.width = 120
	home2.height = 40
	home2.SetSetupRequired(false)
	if strings.Contains(home2.View(), "Setup Required") {
		t.Fatal("banner shown when probe is not misconfigured")
	}
}

// TestLKeyOpensLogin: 'l' in normal mode routes the /login command
// through the message path (the dead-end handle opens the wizard on the
// live app, never on a handleKeys copy), and in insert mode it types
// into the composer.
func TestLKeyOpensLogin(t *testing.T) {
	app := NewApp()
	app.width = 80
	app.height = 24

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if cmd == nil {
		t.Fatal("'l' produced no command")
	}
	msg := cmd()
	uc, ok := msg.(UserCommandMsg)
	if !ok || uc.Command != "/login" {
		t.Fatalf("'l' command = %T %v, want UserCommandMsg{/login}", msg, msg)
	}

	// Feeding the message back must open the wizard on the live app.
	var got string
	app.onUserCommand = func(c string, _ *App) { got = c }
	_, _ = model.(*App).Update(uc)
	if got != "/login" {
		t.Fatalf("message routed %q, want /login", got)
	}

	app2 := NewApp()
	app2.width = 80
	app2.height = 24
	app2.mode = ModeInsert
	app2.chatModel.Focus()
	app2.activeView = viewChat
	app2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if in := app2.chatModel.GetInput(); in != "l" {
		t.Fatalf("insert-mode 'l' input = %q, want 'l'", in)
	}
}

// TestLKeyOpensWizardOnLiveApp pins the copy-clobber bug: the dialog
// must be showing after the full key -> message -> handler chain.
func TestLKeyOpensWizardOnLiveApp(t *testing.T) {
	app := NewApp()
	app.width = 80
	app.height = 24
	app.onUserCommand = func(cmd string, a *App) {
		if cmd == "/login" {
			a.loginDialog.Open(a.width, a.height, "")
		}
	}

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	a := model.(*App)
	if cmd == nil {
		t.Fatal("no command from 'l'")
	}
	a.Update(cmd().(UserCommandMsg))
	if !a.loginDialog.IsShowing() {
		t.Fatal("login dialog not showing after 'l' (state was clobbered)")
	}
}
