package tui

import (
	"fmt"
	"strings"
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
				assertSectionsShowOnce(t, h.Text())
				h.Press("up")
			}
			// Scrolling out of Labs pins the next section's header.
			h.AssertContains("Theme", "Appearance")
			assertSectionsShowOnce(t, h.Text())
		})
	}
}

// assertSectionsShowOnce fails when any section name is on screen twice:
// a header is pinned or inline, never both.
func assertSectionsShowOnce(t *testing.T, frame string) {
	t.Helper()
	for _, section := range []string{"Provider & Connection", "Appearance", "Labs"} {
		if n := strings.Count(frame, section); n > 1 {
			t.Fatalf("section %q shows %d times:\n%s", section, n, frame)
		}
	}
}

// pinnedSection on a two-section list. Lines: 0 header X, 1 row A,
// 2 separator, 3 header Y, 4 row B.
func TestPinnedSection(t *testing.T) {
	settings := []Setting{{Key: "a", Category: "X"}, {Key: "b", Category: "Y"}}
	rowEnd := []int{2, 5}
	headerLine := []int{0, 3}
	for offset, want := range map[int]string{
		0: "",  // header X on screen
		1: "X", // row A on top, its header above the window
		2: "",  // separator on top: row A is gone, header Y follows
		3: "",  // header Y on top
		4: "Y", // row B on top, its header above the window
	} {
		if got := pinnedSection(settings, rowEnd, headerLine, offset); got != want {
			t.Errorf("offset %d: pinned %q, want %q", offset, got, want)
		}
	}
	if got := pinnedSection([]Setting{{Key: "a"}}, []int{1}, []int{-1}, 0); got != "" {
		t.Errorf("uncategorized row pinned %q, want nothing", got)
	}
}
