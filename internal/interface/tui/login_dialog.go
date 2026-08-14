// LoginDialogModel is the modal login wizard: provider, masked API key,
// model. The API key is held in memory and rendered as asterisks —
// typed or pasted — so it never appears in the chat pane, session
// files, or exports. Completion hands the values to the app through
// LoginHandler, which persists them to the encrypted store.

package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LoginStep is one stage of the modal wizard.
type LoginStep int

const (
	LoginStepProvider LoginStep = iota
	LoginStepAPIKey
	LoginStepModel
)

// loginProviders are the selectable providers in the modal.
var loginProviders = []string{"local", "openai", "anthropic", "openrouter", "ollama", "fireworks", "nvidia"}

// LoginHandler receives the completed wizard values on the event loop.
type LoginHandler func(provider, apiKey, model string, a *App)

// LoginDialogModel manages the login overlay.
type LoginDialogModel struct {
	width   int
	height  int
	showing bool
	step    LoginStep

	providerIdx int
	apiKeyBuf   string
	modelBuf    string
	errorMsg    string

	// storedKeyHint is the masked hint of an already-stored key
	// (e.g. "sk-or-…7f75"). When set, the dialog lets the user finish
	// without re-entering the key, keeping the stored secret.
	storedKeyHint string
}

// NewLoginDialog creates the login dialog model.
func NewLoginDialog() LoginDialogModel {
	return LoginDialogModel{step: LoginStepProvider}
}

// Open resets and shows the dialog. storedKeyHint is a masked hint of an
// existing stored key; an empty value means no key is stored yet.
func (m *LoginDialogModel) Open(width, height int, storedKeyHint string) {
	m.width = width
	m.height = height
	m.showing = true
	m.step = LoginStepProvider
	m.providerIdx = 0
	m.apiKeyBuf = ""
	m.modelBuf = ""
	m.errorMsg = ""
	m.storedKeyHint = storedKeyHint
}

// Close hides the dialog.
func (m *LoginDialogModel) Close() {
	m.showing = false
}

// IsShowing reports whether the dialog is visible.
func (m LoginDialogModel) IsShowing() bool {
	return m.showing
}

// provider returns the currently selected provider id.
func (m LoginDialogModel) provider() string {
	return loginProviders[m.providerIdx]
}

// Update handles a key message. Returns (completed, cancelled,
// provider, apiKey, model). completed is true when the wizard finished;
// cancelled is true when the user aborted (Esc).
func (m *LoginDialogModel) Update(msg tea.KeyMsg) (completed, cancelled bool, provider, apiKey, model string) {
	switch m.step {
	case LoginStepProvider:
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
			p := m.provider()
			if p == "local" {
				m.step = LoginStepModel
			} else {
				m.step = LoginStepAPIKey
			}
			m.errorMsg = ""
		case "esc":
			return false, true, "", "", ""
		}

	case LoginStepAPIKey:
		switch msg.String() {
		case "esc":
			return false, true, "", "", ""
		case "enter":
			if strings.TrimSpace(m.apiKeyBuf) == "" && m.storedKeyHint == "" {
				m.errorMsg = "API key cannot be empty."
				return false, false, "", "", ""
			}
			m.step = LoginStepModel
			m.errorMsg = ""
		case "backspace":
			if len(m.apiKeyBuf) > 0 {
				m.apiKeyBuf = m.apiKeyBuf[:len(m.apiKeyBuf)-1]
			}
		default:
			if msg.Paste || msg.Type == tea.KeyRunes {
				m.apiKeyBuf += string(msg.Runes)
			}
		}

	case LoginStepModel:
		switch msg.String() {
		case "esc":
			return false, true, "", "", ""
		case "enter":
			return true, false, m.provider(), strings.TrimSpace(m.apiKeyBuf), strings.TrimSpace(m.modelBuf)
		case "backspace":
			if len(m.modelBuf) > 0 {
				m.modelBuf = m.modelBuf[:len(m.modelBuf)-1]
			}
		default:
			if msg.Paste || msg.Type == tea.KeyRunes {
				m.modelBuf += string(msg.Runes)
			}
		}
	}
	return false, false, "", "", ""
}

// View renders the dialog centered over the app.
func (m LoginDialogModel) View() string {
	if !m.showing {
		return ""
	}

	var body strings.Builder
	switch m.step {
	case LoginStepProvider:
		body.WriteString(HelpTitleStyle.Render("Login - choose provider") + "\n\n")
		for i, p := range loginProviders {
			marker := "  "
			if i == m.providerIdx {
				marker = IndicatorSelected + " "
			}
			body.WriteString(marker + p + "\n")
		}
		body.WriteString("\n" + HelpDimStyle.Render("j/k: navigate  Enter: select  Esc: cancel"))

	case LoginStepAPIKey:
		body.WriteString(HelpTitleStyle.Render("Login - API key") + "\n\n")
		body.WriteString(HelpDimStyle.Render("Provider: "+m.provider()) + "\n\n")
		if m.storedKeyHint != "" {
			body.WriteString(SuccessStyle.Render("Stored key: "+m.storedKeyHint+" (leave empty to keep it)") + "\n\n")
		}
		body.WriteString(HelpDimStyle.Render("Enter API key (masked; safe to paste):") + "\n")
		body.WriteString("  " + PromptStyle.Render(strings.Repeat("*", len(m.apiKeyBuf))+"█") + "\n")
		if m.errorMsg != "" {
			body.WriteString("\n" + ErrorStyle.Render(m.errorMsg) + "\n")
		}
		body.WriteString("\n" + HelpDimStyle.Render("Enter: continue  Esc: cancel"))

	case LoginStepModel:
		body.WriteString(HelpTitleStyle.Render("Login - model") + "\n\n")
		body.WriteString(HelpDimStyle.Render("Provider: "+m.provider()) + "\n\n")
		body.WriteString(HelpDimStyle.Render("Enter model (Enter for default):") + "\n")
		body.WriteString("  " + PromptStyle.Render(m.modelBuf+"█") + "\n")
		body.WriteString("\n" + HelpDimStyle.Render("Enter: finish  Esc: cancel"))
	}

	panel := lipgloss.NewStyle().
		Width(48).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, panel.Render(body.String()))
}
