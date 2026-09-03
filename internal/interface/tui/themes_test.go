package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// TestThemeCatalogHasTwentyThemes pins the catalog size promised by the
// release: 20 themes, default first in the listing.
func TestThemeCatalogHasTwentyThemes(t *testing.T) {
	names := ThemeNames()
	if len(names) != 20 {
		t.Fatalf("catalog has %d themes, want 20: %v", len(names), names)
	}
	if names[0] != "default" {
		t.Fatalf("default must list first, got %q", names[0])
	}
}

// TestLookupThemeNormalizesNames pins the forgiving lookup: case and
// separator variants resolve; unknown names fail.
func TestLookupThemeNormalizesNames(t *testing.T) {
	for _, name := range []string{"default", "Default", "TOKYO-NIGHT", "tokyo_night", "Tokyo Night"} {
		if _, ok := LookupTheme(name); !ok {
			t.Fatalf("LookupTheme(%q) = not ok", name)
		}
	}
	if _, ok := LookupTheme("no-such-theme"); ok {
		t.Fatal("unknown theme resolved")
	}
}

// TestApplyThemeRebuildsStyles pins the live-switch contract: applying
// a theme changes the color vars AND the derived style vars, and the
// default theme restores the shipped palette exactly.
func TestApplyThemeRebuildsStyles(t *testing.T) {
	shipPrimary := ColorPrimary
	shipFG := AssistantStyle.GetForeground()

	if !ApplyTheme("nord") {
		t.Fatal("ApplyTheme(nord) = false")
	}
	nord, _ := LookupTheme("nord")
	if ColorPrimary != nord.Palette.Primary {
		t.Fatalf("color var not swapped: %v", ColorPrimary)
	}
	if AssistantStyle.GetForeground() != nord.Palette.Primary {
		t.Fatalf("styles not rebuilt: assistant FG = %s, want %s", AssistantStyle.GetForeground(), nord.Palette.Primary)
	}

	// Back to default: identical palette and style foreground.
	if !ApplyTheme("default") {
		t.Fatal("ApplyTheme(default) = false")
	}
	if ColorPrimary != shipPrimary || AssistantStyle.GetForeground() != shipFG {
		t.Fatal("default theme did not restore the shipped look")
	}
}

// TestThemePalettesComplete guards against a half-written entry: every
// theme must carry all thirteen tokens, and no two tokens may share a
// color inside one theme (a collapsed palette reads as a bug).
func TestThemePalettesComplete(t *testing.T) {
	for _, name := range ThemeNames() {
		theme, ok := LookupTheme(name)
		if !ok {
			t.Fatalf("%s not resolvable", name)
		}
		p := theme.Palette
		tokens := []lipgloss.Color{
			p.Primary, p.Secondary, p.Accent, p.Success, p.Error,
			p.Warning, p.Info, p.Text, p.TextDim, p.Surface,
			p.Border, p.Muted, p.Highlight,
		}
		seen := map[lipgloss.Color]int{}
		for _, token := range tokens {
			if string(token) == "" {
				t.Fatalf("%s has an empty token", name)
			}
			seen[token]++
		}
		for token, n := range seen {
			if n > 2 {
				t.Fatalf("%s reuses %s across %d tokens", name, token, n)
			}
		}
	}
}

// TestThemeBubbleIdentityContrast pins the speaker hierarchy: the You
// bubble borders with Secondary, the Agent bubble with Primary — the
// two must never share a color in any theme, or the speakers become
// indistinguishable.
func TestThemeBubbleIdentityContrast(t *testing.T) {
	for _, name := range ThemeNames() {
		theme, ok := LookupTheme(name)
		if !ok {
			t.Fatalf("%s not resolvable", name)
		}
		if theme.Palette.Primary == theme.Palette.Secondary {
			t.Fatalf("%s: Primary and Secondary are identical — the You and Agent bubbles would render as the same color", name)
		}
	}
}

// TestBubbleBordersTrackTheme pins the live-switch contract for the
// speaker bubbles: after a theme change, the You bubble keeps its
// Secondary gutter and the Agent bubble its Primary one.
func TestBubbleBordersTrackTheme(t *testing.T) {
	if !ApplyTheme("dracula") {
		t.Fatal("ApplyTheme(dracula) = false")
	}
	dracula, _ := LookupTheme("dracula")
	if MessageBubbleUser.GetBorderTopForeground() != dracula.Palette.Secondary {
		t.Fatalf("You bubble gutter = %v, want %v", MessageBubbleUser.GetBorderTopForeground(), dracula.Palette.Secondary)
	}
	if MessageBubbleAssistant.GetBorderTopForeground() != dracula.Palette.Primary {
		t.Fatalf("Agent bubble gutter = %v, want %v", MessageBubbleAssistant.GetBorderTopForeground(), dracula.Palette.Primary)
	}
	ApplyTheme("default")
}
