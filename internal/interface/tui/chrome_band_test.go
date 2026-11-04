package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// The top and bottom chrome are bands, not floating boxes: the tab bar's
// rule and the composer's rule both run edge to edge, in the theme's
// accent, and the footer below the composer carries no rule of its own.

func TestTabBarRuleTracksThemeAccent(t *testing.T) {
	theme, ok := LookupTheme("dracula")
	if !ok {
		t.Fatal("dracula theme missing")
	}
	if !ApplyTheme("dracula") {
		t.Fatal("could not apply dracula theme")
	}
	defer ApplyTheme("default")

	if got := TabBarStyle.GetBorderBottomForeground(); got != theme.Palette.Primary {
		t.Fatalf("tab bar rule = %v, want the theme accent %v", got, theme.Palette.Primary)
	}
}

func TestFooterHasNoRuleAboveItsText(t *testing.T) {
	if StatusBarStyle.GetBorderTop() || StatusBarStyle.GetBorderBottom() {
		t.Fatal("footer grew a rule; the composer rule is the band's only separator")
	}
}

func TestComposerRuleSpansTheFrame(t *testing.T) {
	app := NewApp()
	app.providerReadiness = 1
	app.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	app.chatModel.AddMessage("user", "hello there")

	lines := strings.Split(SanitizeANSI(app.View()), "\n")
	innerWidth := frameWidth(120)
	wantRule := "│" + strings.Repeat("─", innerWidth) + "│"

	composerRule, rules := -1, 0
	for i, line := range lines {
		if line == wantRule {
			composerRule = i
			rules++
		}
	}
	if rules != 2 {
		t.Fatalf("full-width rules = %d, want the tab bar and the composer:\n%s", rules, strings.Join(lines, "\n"))
	}

	// The rule must be the last one before the input: the footer band
	// has no second rule of its own.
	for _, line := range lines[composerRule+1:] {
		if line == wantRule {
			t.Fatalf("footer grew a second rule:\n%s", strings.Join(lines, "\n"))
		}
	}

	// The input and mode line stay in the inset text column.
	if next := lines[composerRule+1]; !strings.HasPrefix(next, "│  ") {
		t.Fatalf("composer text left the inset column: %q", next)
	}

	// The footer text is inset the same way and has a blank row above it
	// instead of a rule.
	for i, line := range lines {
		if !strings.Contains(line, "[ready]") {
			continue
		}
		if !strings.HasPrefix(line, "│  ") {
			t.Fatalf("footer text left the inset column: %q", line)
		}
		if i == 0 || strings.TrimSpace(strings.Trim(lines[i-1], "│")) != "" {
			t.Fatalf("footer is not preceded by a blank row:\n%s", strings.Join(lines, "\n"))
		}
		if i+1 >= len(lines) || !strings.HasPrefix(lines[i+1], "└") {
			t.Fatalf("footer does not sit on the frame's bottom rule:\n%s", strings.Join(lines, "\n"))
		}
		return
	}
	t.Fatalf("no footer in:\n%s", strings.Join(lines, "\n"))
}
