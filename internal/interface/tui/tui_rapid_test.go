package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"pgregory.net/rapid"

	"github.com/BA-CalderonMorales/agent-harness/internal/interface/commands"
)

func TestTUIRapid_SlashCommandStateTransitions(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		registry := commands.NewSlashRegistry()
		registry.Register("help", "Show help", commands.HelpHandler(registry))
		registry.Register("status", "Show status", commands.StatusHandler(func() string { return "Status: OK" }))
		registry.Register("version", "Show version", commands.VersionHandler("0.3.6", "Git: 123"))
		registry.Register("settings", "Open settings tab", commands.SettingsHandler())

		app := NewApp()
		app.SetUserCommandHandler(func(cmdStr string, a *App) {
			res, handled, err := registry.Handle(cmdStr)
			if handled {
				if err != nil {
					a.chatModel.AddMessage("system", fmt.Sprintf("Error: %v", err))
				} else if res != "" {
					a.chatModel.AddMessage("system", res)
				}
			}
		})
		app.Init()

		var m tea.Model = app
		m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

		// Property 1: Dispatching /settings via UserCommandMsg switches view to viewSettings (3)
		m, _ = m.Update(UserCommandMsg{Command: "/settings"})
		appVal, ok := m.(*App)
		if !ok {
			t.Fatalf("Expected App value, got %T", m)
		}
		if appVal.activeView != viewSettings {
			t.Fatalf("Expected activeView to be viewSettings (3) after /settings, got %d", appVal.activeView)
		}

		viewText := appVal.View()
		if !strings.Contains(viewText, "Settings") {
			t.Fatalf("Expected View() to render Settings tab after /settings, got:\n%s", viewText)
		}

		// Property 2: Palette selection for /settings triggers view switch
		appVal.handlePaletteSelection(&commandInfo{
			Command: "/settings",
			Args:    "",
		})
		appVal2 := appVal
		if appVal2.activeView != viewSettings {
			t.Fatalf("Expected activeView to remain viewSettings (3) after palette /settings")
		}
	})
}

func TestTUIRapid_SettingsNavigationInvariants(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		sModel := NewSettingsModel()
		sModel.SetSettings([]Setting{
			{Key: "provider", Label: "Provider", Value: "local", Category: "Provider & Connection"},
			{Key: "endpoint_url", Label: "Endpoint URL", Value: "http://127.0.0.1:8080/v1", Category: "Provider & Connection"},
			{Key: "model", Label: "Model", Value: "ornith-1.0-9b", Category: "Model & Agent Behavior"},
			{Key: "permissions", Label: "Permission Mode", Value: "workspace-write", Category: "Workspace & Permissions"},
		})
		sModel.Focus()
		sModel.width = 80
		sModel.height = 24

		// Generate random sequence of navigation keys
		keys := rapid.SliceOfN(rapid.SampledFrom([]string{"j", "k", "down", "up", "home", "end"}), 1, 50).Draw(t, "navKeys")

		for _, k := range keys {
			switch k {
			case "home":
				sModel.GotoTop()
			case "end":
				sModel.GotoBottom()
			default:
				m, _ := sModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
				if sm, ok := m.(SettingsModel); ok {
					sModel = sm
				}
			}

			// Invariant: Cursor must stay strictly within [0, len(settings)-1]
			if sModel.cursor < 0 || sModel.cursor >= len(sModel.settings) {
				t.Fatalf("Cursor %d out of bounds [0, %d]", sModel.cursor, len(sModel.settings)-1)
			}
		}

		// Invariant: Section headers are rendered in View()
		view := sModel.View()
		if !strings.Contains(view, "Provider & Connection") || !strings.Contains(view, "Model & Agent Behavior") {
			t.Fatalf("Section headers missing in Settings View():\n%s", view)
		}
	})
}
