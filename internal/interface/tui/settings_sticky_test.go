package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/BA-CalderonMorales/agent-harness/internal/testharness/ttui"
)

// stickySettings ends with a three-row Labs section so the selection can
// scroll inside it. No label or description repeats a section name, so a
// section name on screen can only be its header.
func stickySettings() []Setting {
	return []Setting{
		{Key: "provider", Label: "Provider", Value: "local", Type: "string", Category: "Provider & Connection", Description: "Where requests go."},
		{Key: "tagline", Label: "Home Tagline", Value: "hi", Type: "string", Category: "Appearance", Description: "Shown on Home."},
		{Key: "theme", Label: "Theme", Value: "default", Type: "string", Category: "Appearance", Description: "Palette."},
		{Key: "flag_one", Label: "Flag One", Type: "bool", Category: "Labs", Description: "First flag."},
		{Key: "flag_two", Label: "Flag Two", Type: "bool", Category: "Labs", Description: "Second flag."},
		{Key: "flag_three", Label: "Flag Three", Type: "bool", Category: "Labs", Description: "Third flag."},
	}
}

// The focused row's section header stays in view even when the list
// window is too short to show the inline header above the row.
func TestSettingsSectionHeaderStaysPinned(t *testing.T) {
	for _, size := range [][2]int{{40, 15}, {80, 24}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			app := NewApp()
			app.SetSettings(stickySettings())
			h := ttui.New(t, app)
			h.Resize(size[0], size[1])
			h.Type("5")
			h.Send(tea.KeyMsg{Type: tea.KeyHome})
			h.Press("up") // wraps to the last Labs row

			for _, label := range []string{"Flag Three", "Flag Two", "Flag One"} {
				h.AssertFitsWidth(size[0])
				h.AssertContains(label, "Labs")
				h.Press("up")
			}
			// Scrolling out of Labs pins the next section's header.
			h.AssertContains("Theme", "Appearance")
		})
	}
}
