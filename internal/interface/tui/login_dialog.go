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
var loginProviders = []string{"local", "openai", "anthropic", "openrouter", "ollama", "flm", "fireworks", "nvidia"}

// providerBlurbs give each provider a one-line "what is this / pick when"
// so the first-run user never faces a bare list (requirement #3).
var providerBlurbs = map[string]string{
	"local":      "A local OpenAI-compatible server (llama.cpp/ollama)",
	"openai":     "OpenAI hosted models (GPT-4o family) - needs an API key",
	"anthropic":  "Anthropic Claude models - needs an API key",
	"openrouter": "One key for many models across vendors",
	"ollama":     "Ollama on this machine - no API key needed",
	"flm":        "FastFlowLM on AMD Ryzen AI NPUs - no API key needed",
	"fireworks":  "Fireworks fast hosted inference - needs an API key",
	"nvidia":     "NVIDIA NIM hosted models (nemotron family)",
}

// LoginModelsProvider resolves the model list the wizard's model step
// shows for a candidate provider+key: a live list is the verified
// connection, the static catalog is the honest fallback.
type LoginModelsProvider func(provider, apiKey string) ([]ModelItem, error)

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
	errorMsg    string

	// modelsProvider is wired by the app (SetLoginModelsProvider); the
	// model step is the live-probed picker, not a free-text field.
	modelsProvider LoginModelsProvider
	picker         ModelPickerModel
	probeErr       string

	// stored is the per-provider key set snapshotted from the encrypted
	// store: keys the wizard may reuse without re-entry. In-memory only.
	stored        StoredCredentials
	storedKeyHint string
}

// NewLoginDialog creates the login dialog model.
func NewLoginDialog() LoginDialogModel {
	return LoginDialogModel{step: LoginStepProvider}
}

// SetModelsProvider wires the live model probe into the wizard's model
// step (the app feeds it at boot, mirroring SetLoginHandler).
// StoredCredentials is the encrypted store's per-provider key set,
// snapshotted for the wizard. In-memory only — it never persists to the
// transcript. primaryKey is the most recently used provider's key, for
// the masked hint.
type StoredCredentials struct {
	keys       map[string]string
	primaryKey string
}

// NewStoredCredentials builds the wizard's stored-key snapshot from the
// encrypted store's per-provider keys and the active provider's key.
func NewStoredCredentials(keys map[string]string, primaryKey string) StoredCredentials {
	if keys == nil {
		keys = map[string]string{}
	}
	return StoredCredentials{keys: keys, primaryKey: primaryKey}
}

func (m *LoginDialogModel) SetModelsProvider(provider LoginModelsProvider) {
	m.modelsProvider = provider
}

// primaryKey returns the most recently used provider's key, for the
// masked hint.
func (s StoredCredentials) Primary() string { return s.primaryKey }

// Open resets and shows the dialog. stored carries the per-provider
// keys from the encrypted store: a stored key skips the key step only
// for its own provider, and the live model probe uses it instead of an
// empty key.
func (m *LoginDialogModel) Open(width, height int, stored StoredCredentials) {
	m.width = width
	m.height = height
	m.showing = true
	m.step = LoginStepProvider
	m.providerIdx = 0
	m.apiKeyBuf = ""
	m.errorMsg = ""
	m.probeErr = ""
	m.picker = NewModelPicker()
	m.stored = stored
	m.storedKeyHint = maskKey(stored.primaryKey)
}

// keyHint masks a key for display.
func maskKey(key string) string {
	if key == "" {
		return ""
	}
	const tailLen = 4
	if len(key) <= tailLen+1 {
		return "…" + key
	}
	prefix := key
	if len(prefix) > 6 {
		prefix = prefix[:6]
	}
	return prefix + "…" + key[len(key)-tailLen:]
}

// probeKey resolves the key the model-step probe should use: a typed
// key wins, then the stored key for THIS provider (per-provider key
// set — switching providers never loses the keys you already entered).
func (m *LoginDialogModel) probeKey() string {
	if strings.TrimSpace(m.apiKeyBuf) != "" {
		return strings.TrimSpace(m.apiKeyBuf)
	}
	if key, ok := m.stored.keys[m.provider()]; ok {
		return key
	}
	return ""
}

// finishKey resolves the key the login completes with.
func (m *LoginDialogModel) finishKey() string {
	return m.probeKey()
}

// loadModels fetches the live model list for the candidate provider+key
// and populates the picker. A failed probe surfaces the error next to
// the static catalog - never a silent default.
func (m *LoginDialogModel) loadModels() {
	m.probeErr = ""
	if m.modelsProvider == nil {
		return
	}
	models, err := m.modelsProvider(m.provider(), m.probeKey())
	if err != nil {
		m.probeErr = err.Error()
	}
	if len(models) > 0 {
		// Size the picker viewport for the dialog panel: Width drives the
		// name truncation, Height the scroll window and cursor sync.
		vpW := m.width - 12
		if vpW < 30 {
			vpW = 30
		}
		vpH := m.height - 14
		if vpH < 5 {
			vpH = 5
		}
		if vpH > 12 {
			vpH = 12
		}
		m.picker.viewport = newViewport(vpW, vpH)
		m.picker.SetTitle("Models - " + m.provider())
		m.picker.SetModels(models)
	}
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
			// The key step is skipped only when local needs none or the
			// stored key belongs to THIS provider — a key minted for
			// another provider probed with 401s and dead-ended the
			// login.
			if p == "local" || m.hasStoredKey(p) {
				m.step = LoginStepModel
				m.loadModels()
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
			if strings.TrimSpace(m.apiKeyBuf) == "" {
				m.errorMsg = "API key cannot be empty."
				return false, false, "", "", ""
			}
			m.step = LoginStepModel
			m.loadModels()
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
		// The model step is the live-probed picker: type to filter,
		// j/k to navigate, Enter to finish, Esc to cancel. Enter with
		// no selection keeps completeLogin's empty->default fallback
		// (the picker itself needs a selection to close, so an empty
		// list is completed here).
		if msg.String() == "enter" && len(m.picker.filtered) == 0 {
			return true, false, m.provider(), m.finishKey(), ""
		}
		closed, _ := m.picker.Update(msg)
		if closed {
			if selected := m.picker.SelectedModel(); selected != nil {
				return true, false, m.provider(), m.finishKey(), selected.ID
			}
			if msg.String() == "esc" || msg.String() == "q" {
				return false, true, "", "", ""
			}
			return true, false, m.provider(), strings.TrimSpace(m.apiKeyBuf), ""
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
			body.WriteString(HelpDimStyle.Render("    "+providerBlurbs[p]) + "\n")
		}
		body.WriteString("\n" + HelpDimStyle.Render("j/k: navigate  Enter: select  Esc: cancel"))

	case LoginStepAPIKey:
		body.WriteString(HelpTitleStyle.Render("Login - API key") + "\n\n")
		body.WriteString(HelpDimStyle.Render("Provider: "+m.provider()) + "\n\n")
		body.WriteString(HelpDimStyle.Render("Enter API key (masked; safe to paste):") + "\n")
		body.WriteString("  " + PromptStyle.Render(strings.Repeat("*", len(m.apiKeyBuf))+"█") + "\n")
		if m.errorMsg != "" {
			body.WriteString("\n" + ErrorStyle.Render(m.errorMsg) + "\n")
		}
		body.WriteString("\n" + HelpDimStyle.Render("Enter: continue  Esc: cancel"))

	case LoginStepModel:
		body.WriteString(HelpTitleStyle.Render("Login - model") + "\n")
		body.WriteString(HelpDimStyle.Render("Provider: "+m.provider()+"  ·  the list below is live from the endpoint") + "\n")
		if m.probeErr != "" {
			body.WriteString(ErrorStyle.Render("[!] Could not reach endpoint: "+m.probeErr) + "\n")
			body.WriteString(HelpDimStyle.Render("    Showing the static catalog; Enter still finishes.") + "\n")
		}
		body.WriteString("\n")
		body.WriteString(m.picker.viewport.View())
		body.WriteString("\n" + HelpDimStyle.Render("Type to filter  j/k: navigate  Enter: finish  Esc: cancel"))
	}

	// The provider step lists blurbs and needs a wider panel; the model
	// step hosts the picker viewport. lipgloss truncates content wider
	// than the style width, so the panel must fit its step's longest line.
	panelWidth := 64
	if m.step == LoginStepAPIKey {
		panelWidth = 48
	} else if m.step == LoginStepModel {
		// Full-width panel: live model ids (accounts/fireworks/models/...)
		// are far longer than a fixed 54-col panel can show.
		panelWidth = m.width - 4
		if panelWidth < 54 {
			panelWidth = 54
		}
	}

	panel := lipgloss.NewStyle().
		Width(panelWidth).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, panel.Render(body.String()))
}

// hasStoredKey reports whether the store holds a key for this provider.
func (m *LoginDialogModel) hasStoredKey(provider string) bool {
	_, ok := m.stored.keys[provider]
	return ok && m.stored.keys[provider] != ""
}
