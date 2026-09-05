package tui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/diag"
)

// View renders the TUI. A panic anywhere in the render tree must never
// kill the program (bubbletea's own recovery exits): auto mode can be
// mid-task, and a dead TUI strands the work. The recover degrades to a
// one-frame error screen; the next Update usually renders clean.
func (a App) View() string {
	defer func() {
		if r := recover(); r != nil {
			diag.Panic("tui.view", r)
		}
	}()
	return a.view()
}

func (a App) view() string {
	if a.width == 0 {
		return "  Initializing..."
	}

	if a.showHelp {
		return a.helpModel.View()
	}

	// Render approval dialog overlay first (if visible)
	if a.approvalDialog.IsVisible() {
		return a.approvalDialog.View()
	}

	tabBar := a.renderTabBar()
	content := a.renderActiveView()
	statusBar := a.renderStatusBar()

	// Render overlays on top (they fill the screen via lipgloss.Place)
	if a.loginDialog.IsShowing() {
		return a.loginDialog.View()
	}
	if a.providerPicker.IsShowing() {
		return a.providerPicker.View()
	}
	if a.commandPalette.IsShowing() {
		return a.commandPalette.View(a.width, a.height)
	}
	if a.modelPicker.IsShowing() {
		return a.modelPicker.View(a.width, a.height)
	}
	if a.exportPicker.visible {
		return a.exportPicker.View(a.width, a.height)
	}

	// Terminal-native: no painted surface anywhere. Structure comes
	// from rules, spacing, and color — the terminal's own background is
	// the app's background, which is the leanest, easiest-on-the-eyes
	// surface there is.
	return lipgloss.JoinVertical(lipgloss.Left, tabBar, content, statusBar)
}

// ---------------------------------------------------------------------------
// Tab bar rendering - Golazo-inspired centered design
// ---------------------------------------------------------------------------

// tabBarTier is one degradation step of the tab bar: a phone pane
// cannot show five padded labels, so the bar sheds padding, then
// spelling, then breadth — and never wraps. Tier 0 is the desktop
// design, byte-identical to the pane that has always worked.
type tabBarTier struct {
	pad        int
	short      bool
	activeOnly bool
}

var tabBarTiers = []tabBarTier{
	{pad: 2}, {pad: 1}, {pad: 1, short: true}, {activeOnly: true},
}

// shortTabLabels are the narrow-pane spellings; distinct pairs, no
// iconography, still legible at a glance.
var shortTabLabels = [viewCount]string{"Ho", "Ch", "Se", "Lo", "St"}

func (a App) renderTabLine(tier tabBarTier) string {
	var tabs []string

	for i := viewID(0); i < viewCount; i++ {
		if tier.activeOnly && i != a.activeView {
			continue
		}
		style := TabNormal
		indicator := " "
		if i == a.activeView {
			style = TabActive
			indicator = IndicatorSelected
		}
		label := indicator + viewLabels[i]
		if tier.short {
			label = indicator + shortTabLabels[i]
		}
		// Show activity indicator for tabs with unseen updates
		if a.tabActivity[i] && i != a.activeView {
			label += " " + InfoStyle.Render(IndicatorActive)
		}
		if tier.pad != 2 {
			style = TabNormal.Padding(0, tier.pad)
			if i == a.activeView {
				style = TabActive.Padding(0, tier.pad)
			}
		}
		tabs = append(tabs, style.Render(label))
	}

	return lipgloss.JoinHorizontal(lipgloss.Center, tabs...)
}

func (a App) renderTabBar() string {
	// The bar must never wrap: walk the tiers until the line fits the
	// pane. Tier 0 fits on every desktop pane, so desktop renders the
	// design it has always had.
	line := ""
	for _, tier := range tabBarTiers {
		line = a.renderTabLine(tier)
		if lipgloss.Width(line) <= a.width {
			break
		}
	}

	// Center the tabs in the available width
	centeredTabs := lipgloss.PlaceHorizontal(a.width, lipgloss.Center, line)

	// Apply tab bar styling with top padding for breathing room
	return TabBarStyle.Width(a.width).PaddingTop(1).Render(centeredTabs)
}

// ---------------------------------------------------------------------------
// Active view content
// ---------------------------------------------------------------------------

// gutterFor is the horizontal breathing room around a tab's content:
// a phone pane should not press text against the device's edges.
// Desktop panes get none — their layout is frozen.
func gutterFor(width int) int {
	if !isMobilePane(width) {
		return 0
	}
	if width >= 50 {
		return 2
	}
	return 1
}

func (a App) renderActiveView() string {
	// Reserve space for the fixed chrome: tab bar (3 with padding and
	// border) + status bar (3 with its top and bottom padding). The
	// reserve must match the chrome's real height — one row short and
	// the pane scrolls a row of the tab bar off the top.
	contentHeight := a.height - 6
	if contentHeight < 1 {
		contentHeight = 1
	}

	// The gutter insets the content on phone panes; sub-models already
	// rendered to the inset width (resize shrank it), so the padding
	// and the content agree. Desktop: gutter 0, byte-identical output.
	gutter := gutterFor(a.width)
	var styled lipgloss.Style
	if gutter > 0 {
		styled = lipgloss.NewStyle().Padding(0, gutter)
	} else {
		styled = lipgloss.NewStyle()
	}

	// Height pads the pane to its budget; MaxHeight clips anything
	// that exceeds it — a clipped row is graceful, an overflowing
	// frame leaves ghost duplicates of the bottom chrome behind.
	switch a.activeView {
	case viewHome:
		return styled.Height(contentHeight).MaxHeight(contentHeight).Render(a.homeModel.View())
	case viewChat:
		return styled.Height(contentHeight).MaxHeight(contentHeight).Render(a.chatModel.View())
	case viewSessions:
		return styled.Height(contentHeight).MaxHeight(contentHeight).Render(a.sessionsModel.View())
	case viewLogs:
		return styled.Height(contentHeight).MaxHeight(contentHeight).Render(a.logsModel.View())
	case viewSettings:
		return styled.Height(contentHeight).MaxHeight(contentHeight).Render(a.settingsModel.View())
	}
	return ""
}

// ---------------------------------------------------------------------------
// Status bar rendering
// ---------------------------------------------------------------------------

// renderStatusBar renders the bottom bar: workspace path on the left,
// context usage / cost / keybind hints on the right, centered like the
// composer column above it.
