package tui

import (
	"github.com/charmbracelet/lipgloss"
)

func (a App) View() string {
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
	if a.commandPalette.IsShowing() {
		return a.commandPalette.View(a.width, a.height)
	}
	if a.modelPicker.IsShowing() {
		return a.modelPicker.View(a.width, a.height)
	}

	return lipgloss.JoinVertical(lipgloss.Left, tabBar, content, statusBar)
}

// ---------------------------------------------------------------------------
// Tab bar rendering - Golazo-inspired centered design
// ---------------------------------------------------------------------------

func (a App) renderTabBar() string {
	var tabs []string

	for i := viewID(0); i < viewCount; i++ {
		style := TabNormal
		indicator := " "
		if i == a.activeView {
			style = TabActive
			indicator = IndicatorSelected
		}
		label := indicator + viewLabels[i]
		// Show activity indicator for tabs with unseen updates
		if a.tabActivity[i] && i != a.activeView {
			label += " " + InfoStyle.Render(IndicatorActive)
		}
		tabs = append(tabs, style.Render(label))
	}

	// Join tabs with spacing
	tabsContent := lipgloss.JoinHorizontal(lipgloss.Center, tabs...)

	// Center the tabs in the available width
	centeredTabs := lipgloss.PlaceHorizontal(a.width, lipgloss.Center, tabsContent)

	// Apply tab bar styling with top padding for breathing room
	return TabBarStyle.Width(a.width).PaddingTop(1).Render(centeredTabs)
}

// ---------------------------------------------------------------------------
// Active view content
// ---------------------------------------------------------------------------

func (a App) renderActiveView() string {
	// Reserve space for tab bar (3 with padding) + status bar (2 with padding)
	contentHeight := a.height - 5
	if contentHeight < 1 {
		contentHeight = 1
	}

	switch a.activeView {
	case viewHome:
		return lipgloss.NewStyle().Height(contentHeight).Render(a.homeModel.View())
	case viewChat:
		return lipgloss.NewStyle().Height(contentHeight).Render(a.chatModel.View())
	case viewSessions:
		return lipgloss.NewStyle().Height(contentHeight).Render(a.sessionsModel.View())
	case viewSettings:
		return lipgloss.NewStyle().Height(contentHeight).Render(a.settingsModel.View())
	}
	return ""
}

// ---------------------------------------------------------------------------
// Status bar rendering
// ---------------------------------------------------------------------------

// renderStatusBar renders the bottom bar: workspace path on the left,
// context usage / cost / keybind hints on the right, centered like the
// composer column above it.
