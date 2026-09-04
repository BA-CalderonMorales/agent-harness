package tui

import (
	"math"
	"strconv"
	"strings"
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

// TestThemeBubbleIdentityPerceptualContrast pins the perceived
// hierarchy, not just byte equality. A one-character gutter reads as
// the same color when the two tokens sit in the same hue family at
// similar lightness — ice (ΔHue 2°, ΔL 0.08), ember (10°, 0.05),
// everforest (51°, 0.01), gruvbox (53°, 0.04), and midnight (53°, 0.14)
// all shipped that way and read as one color on screen. Distinctness is
// a big hue gap OR a big lightness step (forest passes on lightness
// alone: same green, much paler).
func TestThemeBubbleIdentityPerceptualContrast(t *testing.T) {
	const minHueGap = 70.0
	const minLightStep = 0.14
	for _, name := range ThemeNames() {
		theme, _ := LookupTheme(name)
		ph, _, pl := hexToHSL(theme.Palette.Primary)
		sh, _, sl := hexToHSL(theme.Palette.Secondary)
		dHue := hueDistance(ph, sh)
		dLight := absFloat(pl - sl)
		if dHue < minHueGap && dLight < minLightStep {
			t.Fatalf("%s: Primary/Secondary too close to distinguish (ΔHue %.0f°, ΔL %.2f)", name, dHue, dLight)
		}
	}
}

func hexToHSL(hexs lipgloss.Color) (hue, sat, light float64) {
	r, g, b := hexRGB(hexs)
	h, l, s := rgbToHLS(r, g, b)
	return h * 360, s, l
}

func hexRGB(hexs lipgloss.Color) (float64, float64, float64) {
	hex := string(hexs)
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return 0, 0, 0
	}
	parse := func(s string) float64 {
		v, _ := strconv.ParseInt(s, 16, 32)
		return float64(v) / 255
	}
	return parse(hex[0:2]), parse(hex[2:4]), parse(hex[4:6])
}

// TestThemeInstructionTextContrast pins the accessibility floor for the
// instruction layer: Muted (mode line, hints, descriptions) and TextDim
// (timestamps, secondary labels) must hold WCAG AA normal (4.5:1)
// against each theme's own surface, in every theme. The pre-fix catalog
// measured Muted as low as 1.69:1 (nord) — instruction text below the
// readable floor everywhere.
func TestThemeInstructionTextContrast(t *testing.T) {
	for _, name := range ThemeNames() {
		theme, _ := LookupTheme(name)
		sr, sg, sb := hexRGB(theme.Palette.Surface)
		surface := [3]float64{sr, sg, sb}
		for tokenName, token := range map[string]lipgloss.Color{
			"Muted":   theme.Palette.Muted,
			"TextDim": theme.Palette.TextDim,
		} {
			tr, tg, tb := hexRGB(token)
			r := contrastRatio([3]float64{tr, tg, tb}, surface)
			if r < 4.5 {
				t.Errorf("%s: %s contrast %.2f:1, want >= 4.5:1", name, tokenName, r)
			}
		}
	}
}

// contrastRatio computes the WCAG 2.x contrast ratio between two RGB
// triples (0..1).
func contrastRatio(a, b [3]float64) float64 {
	lum := func(c [3]float64) float64 {
		lin := func(v float64) float64 {
			if v <= 0.03928 {
				return v / 12.92
			}
			return math.Pow((v+0.055)/1.055, 2.4)
		}
		return 0.2126*lin(c[0]) + 0.7152*lin(c[1]) + 0.0722*lin(c[2])
	}
	la, lb := lum(a), lum(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func hueDistance(a, b float64) float64 {
	d := math.Abs(a-b) - math.Floor(math.Abs(a-b)/360)*360
	if d > 180 {
		d = 360 - d
	}
	return d
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
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

// rgbToHLS converts 0..1 RGB to hue (0..1), lightness, saturation.
func rgbToHLS(r, g, b float64) (h, l, s float64) {
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	l = (max + min) / 2
	if max == min {
		return 0, l, 0
	}
	d := max - min
	if l > 0.5 {
		s = d / (2 - max - min)
	} else {
		s = d / (max + min)
	}
	switch max {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return h / 6, l, s
}
