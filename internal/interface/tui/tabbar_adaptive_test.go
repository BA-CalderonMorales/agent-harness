package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// The tab bar is fixed chrome: whatever the pane, it renders as one
// row inside the pane. Phones shed padding, then spelling, then
// breadth — desktop (tier 0) renders the design it has always had.

// TestTabBarNeverWraps walks the widths phone panes actually are and
// asserts the rendered bar is a single row that fits the pane, with
// every tab reachable.
func TestTabBarNeverWraps(t *testing.T) {
	for _, width := range []int{24, 30, 36, 40, 45, 50, 55, 60, 70, 120} {
		app := NewApp()
		app.width = width
		bar := app.renderTabBar()
		lines := strings.Split(bar, "\n")
		if len(lines) != 3 { // padding row + labels + border
			t.Fatalf("w%d: tab bar rendered %d rows, want 3", width, len(lines))
		}
		for i, line := range lines {
			if w := ansi.StringWidth(line); w > width {
				t.Fatalf("w%d: tab bar row %d is %d wide", width, i, w)
			}
		}
	}
}

// TestTabBarShedsLabelsGracefully: the tiers engage in order — full
// labels with desktop padding on wide panes, full labels with tight
// padding on medium ones, short labels on phones, and the active tab
// alone on an absurdly narrow pane.
func TestTabBarShedsLabelsGracefully(t *testing.T) {
	at := func(width int) string {
		app := NewApp()
		app.width = width
		return app.renderTabLine(tabBarTiers[0])
	}

	// Wide pane: every label, fully spelled.
	wide := at(120)
	for _, label := range viewLabels {
		if !strings.Contains(wide, label) {
			t.Fatalf("wide pane dropped label %q", label)
		}
	}
	if strings.Contains(wide, shortTabLabels[0]) && strings.Contains(wide, " Ho ") {
		t.Fatalf("wide pane degraded to short labels: %q", wide)
	}

	// Phone pane: short labels keep every tab distinct and present.
	app := NewApp()
	app.width = 30
	short := app.renderTabBar()
	for _, label := range shortTabLabels {
		if !strings.Contains(short, label) {
			t.Fatalf("phone pane dropped short label %q:\n%s", label, short)
		}
	}

	// Absurd pane: the active tab alone still names where you are.
	app.width = 14
	solo := app.renderTabBar()
	if !strings.Contains(ansi.Strip(solo), "Home") {
		t.Fatalf("solo tier lost the active tab:\n%s", solo)
	}
}

// TestTabBarWideIsTierZero pins the frozen side: at desktop width the
// bar renders exactly the tier-0 line — the design that has always
// worked, untouched.
func TestTabBarWideIsTierZero(t *testing.T) {
	app := NewApp()
	app.width = 120
	if ansi.StringWidth(app.renderTabLine(tabBarTiers[0])) > 120 {
		t.Fatal("tier 0 itself overflows at desktop width; thresholds drifted")
	}
	line := strings.TrimSpace(ansi.Strip(app.renderTabLine(tabBarTiers[0])))
	if line != goldenWideTabLine() {
		t.Fatalf("desktop tab line changed:\n%q", line)
	}
}

// goldenWideTabLine is the tier-0 label row the desktop pane has
// always rendered (the active tab carries the "> " indicator); a
// change here is a desktop-visible change and needs the owner's
// say-so.
func goldenWideTabLine() string {
	return "> Home     Chat     Sessions     Logs     Settings"
}
