// ProviderPickerModel is the standalone provider-switch modal: pick a
// provider, and the app opens the model picker with that provider's full
// model list. It is deliberately separate from the login dialog, so a
// stored key is retained across provider switches and the user never has
// to re-authenticate just to try another provider.

package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ProviderPickHandler receives the chosen provider on the event loop.
type ProviderPickHandler func(provider string, a *App)

// ProviderPickerModel manages the provider-switch overlay.
type ProviderPickerModel struct {
	width       int
	height      int
	showing     bool
	providerIdx int
}

// NewProviderPicker creates the provider picker model.
func NewProviderPicker() ProviderPickerModel {
	return ProviderPickerModel{}
}

// Open resets and shows the picker.
func (m *ProviderPickerModel) Open(width, height int) {
	m.width = width
	m.height = height
	m.showing = true
	m.providerIdx = 0
}

// Close hides the picker.
func (m *ProviderPickerModel) Close() {
	m.showing = false
}

// IsShowing reports whether the picker is visible.
func (m ProviderPickerModel) IsShowing() bool {
	return m.showing
}

// provider returns the currently selected provider id.
func (m ProviderPickerModel) provider() string {
	return loginProviders[m.providerIdx]
}

// Update handles a key message. Returns (completed, cancelled, provider).
func (m *ProviderPickerModel) Update(msg tea.KeyMsg) (completed, cancelled bool, provider string) {
	switch msg.String() {
	case "up", "k":
		if m.providerIdx > 0 {
			m.providerIdx--
		}
	case "down", "j":
		if m.providerIdx < len(loginProviders)-1 {
			m.providerIdx++
		}
	case "enter", " ":
		return true, false, m.provider()
	case "esc":
		return false, true, ""
	}
	return false, false, ""
}

// View renders the picker centered over the app.
func (m ProviderPickerModel) View() string {
	if !m.showing {
		return ""
	}

	var body strings.Builder
	body.WriteString(HelpTitleStyle.Render("Switch provider") + "\n\n")
	for i, p := range loginProviders {
		marker := "  "
		if i == m.providerIdx {
			marker = IndicatorSelected + " "
		}
		body.WriteString(marker + p + "\n")
	}
	body.WriteString("\n" + HelpDimStyle.Render("j/k: navigate  Enter: pick + models  Esc: cancel"))

	panel := lipgloss.NewStyle().
		Width(48).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, panel.Render(body.String()))
}
