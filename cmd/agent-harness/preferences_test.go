package main

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"os"
	"testing"
)

func TestThemeSurvivesSettingsReload(t *testing.T) {
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", t.TempDir())
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "local", Model: "chosen", Theme: "nord"}, "chosen")
	app.cwd = t.TempDir()
	app.commitConfigChange()
	loaded, err := config.NewLayeredLoader(app.cwd).Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Theme != "nord" {
		t.Fatalf("theme = %q, want nord", loaded.Theme)
	}
}

func TestNewChatsKeepPreferredModelAfterVisitingHistory(t *testing.T) {
	for _, entry := range []string{"home", "sessions"} {
		t.Run(entry, func(t *testing.T) {
			app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "local", Model: "chosen"}, "historical")
			ui := tui.NewApp()
			if entry == "home" {
				(&tuiHomeDelegate{app: app, tuiApp: ui}).OnNewChat()
			} else {
				(&tuiSessionsDelegate{app: app, tuiApp: ui}).OnSessionNew()
			}
			if app.session.Model != "chosen" {
				t.Fatalf("new chat model = %q, want chosen", app.session.Model)
			}
		})
	}
}

func TestUnrelatedSettingsKeepPreferredModel(t *testing.T) {
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", t.TempDir())
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "local", Model: "chosen", Theme: "nord"}, "historical")
	app.cwd = t.TempDir()
	app.commitConfigChange()
	loaded, err := config.NewLayeredLoader(app.cwd).Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Model != "chosen" {
		t.Fatalf("saved model = %q, want chosen", loaded.Model)
	}
}

func TestResumeKeepsPreferredModelAndHonorsPin(t *testing.T) {
	for _, pinned := range []bool{false, true} {
		t.Run(map[bool]string{false: "history", true: "environment"}[pinned], func(t *testing.T) {
			app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "local", Model: "chosen", ModelPinned: pinned}, "historical")
			app.session.AddMessage(types.Message{Role: types.RoleUser, Content: []types.ContentBlock{types.TextBlock{Text: "hello"}}})
			if _, err := app.sessionManager.SaveCurrent(); err != nil {
				t.Fatal(err)
			}
			app.cwd, _ = os.Getwd()
			if err := app.initSession(); err != nil {
				t.Fatal(err)
			}
			if app.config.Model != "chosen" {
				t.Fatalf("default changed to %q", app.config.Model)
			}
			want := "historical"
			if pinned {
				want = "chosen"
			}
			if app.session.Model != want {
				t.Fatalf("resumed model = %q, want %q", app.session.Model, want)
			}
		})
	}
}

func TestCredentialsDoNotReplacePreferredModel(t *testing.T) {
	app := &App{config: &config.LayeredConfig{Model: "chosen"}}
	app.applySecureConfig(&config.SecureConfig{Model: "stale-login-model"})
	if app.config.Model != "chosen" {
		t.Fatalf("model = %q, want chosen", app.config.Model)
	}
}
