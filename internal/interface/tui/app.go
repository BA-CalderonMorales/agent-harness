// Main TUI application with tab-based navigation
// Inspired by lumina-bot's exceptional TUI experience

package tui

import (
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/diag"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/pkg/git"
	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// View identifiers
// ---------------------------------------------------------------------------
type viewID int

const (
	viewHome viewID = iota
	viewChat
	viewSessions
	viewSettings
	viewCount
)

var viewLabels = [viewCount]string{
	"Home", "Chat", "Sessions", "Settings",
}

// ---------------------------------------------------------------------------
// Mode represents the input mode (vim-like)
// ---------------------------------------------------------------------------
type Mode int

const (
	ModeInsert Mode = iota
	ModeNormal
)

// ---------------------------------------------------------------------------
// App is the top-level Bubble Tea model
// ---------------------------------------------------------------------------
type App struct {
	// Dimensions
	width  int
	height int

	// Navigation
	activeView viewID
	mode       Mode

	// Sub-models
	homeModel      *HomeModel
	chatModel      ChatModel
	sessionsModel  SessionsModel
	settingsModel  SettingsModel
	approvalDialog ApprovalDialogModel

	// UI state
	showHelp       bool
	helpModel      Help
	commandPalette CommandPaletteModel
	modelPicker    ModelPickerModel
	providerPicker ProviderPickerModel
	loginDialog    LoginDialogModel
	tabActivity    [viewCount]bool

	// Status
	statusMessage string
	statusType    string // "info", "success", "error", "warning"
	provider      string
	effortProfile string
	workspacePath string
	workspaceName string

	// Telemetry for the bottom bar: context usage and session cost
	estTokens  int
	contextLen int
	costTotal  float64

	// systemLog is the durable, capped list of system messages shown at the
	// bottom of the settings page.
	systemLog []string

	// Provider readiness
	providerReadiness    int // 0=checking, 1=ready, 2=warning, 3=unavailable, 4=misconfigured
	providerReadinessMsg string
	providerReadinessGen int // generation counter to discard stale results

	// External message channel for async updates
	msgChan chan tea.Msg

	// Handlers for user actions (set by main.go)
	onUserSubmit   func(string, *App)
	onUserCommand  func(string, *App)
	onGitContext   func(*git.Context, *App)
	onLogin        LoginHandler
	onProviderPick ProviderPickHandler

	// Agent cancellation context
	agentCancelFunc context.CancelFunc
}

// NewApp creates a new TUI application.
func NewApp() *App {
	home := NewHomeModel()
	app := &App{
		activeView:     viewHome,
		mode:           ModeNormal,
		homeModel:      &home,
		chatModel:      NewChatModel(),
		sessionsModel:  NewSessionsModel(),
		settingsModel:  NewSettingsModel(),
		approvalDialog: NewApprovalDialog(),
		helpModel:      NewHelp(),
		commandPalette: NewCommandPalette(),
		modelPicker:    NewModelPicker(),
		providerPicker: NewProviderPicker(),
		loginDialog:    NewLoginDialog(),
		msgChan:        make(chan tea.Msg, 64),
	}
	app.chatModel.SetModeLabel("navigate")
	app.chatModel.Blur()
	app.focusActive()
	app.homeModel.Init()
	return app
}

// SetAgentCancelFunc sets the cancellation function for the current agent execution
func (a *App) SetAgentCancelFunc(cancel context.CancelFunc) {
	a.agentCancelFunc = cancel
}

// CancelAgent cancels the current agent execution
func (a *App) CancelAgent() {
	if a.agentCancelFunc != nil {
		a.agentCancelFunc()
		a.agentCancelFunc = nil
	}
}

// SetUserSubmitHandler sets the handler for user message submissions.
func (a *App) SetUserSubmitHandler(handler func(string, *App)) {
	a.onUserSubmit = handler
}

// SetUserCommandHandler sets the handler for slash commands.
func (a *App) SetUserCommandHandler(handler func(string, *App)) {
	a.onUserCommand = handler
}

// SetGitContextHandler sets the handler for late-arriving git context;
// it runs on the event loop, so the receiver may mutate app state safely.
func (a *App) SetGitContextHandler(handler func(*git.Context, *App)) {
	a.onGitContext = handler
}

// SetLoginHandler sets the handler that receives completed login wizard
// values (provider, api key, model) on the event loop.
func (a *App) SetLoginHandler(handler LoginHandler) {
	a.onLogin = handler
}

// SetLoginModelsProvider wires the login wizard's model step to the app's
// live model probe: the wizard shows the actual models the candidate
// endpoint can serve (verified connection) with the static catalog as the
// honest fallback.
func (a *App) SetLoginModelsProvider(provider LoginModelsProvider) {
	a.loginDialog.SetModelsProvider(provider)
}

// OpenLoginDialog opens the modal login wizard. storedKeyHint is a masked
// hint of an already-stored key (empty when none exists); the dialog then
// lets the user finish without re-entering the key.
func (a *App) OpenLoginDialog(storedKeyHint string) {
	a.loginDialog.Open(a.width, a.height, storedKeyHint)
}

// SetProviderPickHandler sets the handler that receives the provider
// chosen in the provider-switch modal, on the event loop.
func (a *App) SetProviderPickHandler(handler ProviderPickHandler) {
	a.onProviderPick = handler
}

// OpenProviderPicker opens the provider-switch modal.
func (a *App) OpenProviderPicker() {
	a.providerPicker.Open(a.width, a.height)
}

// ShowModelPicker populates and opens the model picker for the given
// provider, listing the full model set so the user never has to go
// digging for a model.
func (a *App) ShowModelPicker(models []ModelItem, provider string) {
	a.modelPicker.SetTitle("Models - " + provider)
	a.modelPicker.SetModels(models)
	a.modelPicker.Open(a.width, a.height)
}

// SetSessionsDelegate sets the sessions handler delegate.
func (a *App) SetSessionsDelegate(delegate SessionsDelegate) {
	a.sessionsModel.SetDelegate(delegate)
}

// SetSettingsDelegate sets the settings handler delegate.
func (a *App) SetSettingsDelegate(delegate SettingsDelegate) {
	a.settingsModel.SetDelegate(delegate)
}

// SetHomeDelegate sets the home handler delegate.
func (a *App) SetHomeDelegate(delegate HomeDelegate) {
	a.homeModel.SetDelegate(delegate)
}

// SetChatDelegate sets the chat handler delegate.
func (a *App) SetChatDelegate(delegate ChatDelegate) {
	a.chatModel.SetDelegate(delegate)
}

// Send sends a message to the TUI from external goroutines.
// This is the key method that enables async agent loop integration.
func (a *App) Send(msg tea.Msg) {
	select {
	case a.msgChan <- msg:
	default:
		// Channel full, drop message. Dropped streaming events surface
		// as frozen output with no cause — record the drop so a frozen
		// chat traces back here in seconds.
		diag.Errorf("tui.send.drop", "message channel full, dropped %T", msg)
	}
}

// StartProviderProbe starts an async provider readiness probe.
// It returns a generation counter that can be used to discard stale results.
func (a *App) StartProviderProbe(prober llm.ProviderProber) int {
	a.providerReadinessGen++
	gen := a.providerReadinessGen
	a.providerReadiness = 0 // ProviderChecking
	a.providerReadinessMsg = "checking provider..."

	go func() {
		ctx := context.Background()
		readiness, msg := prober.Probe(ctx)
		a.Send(ProviderReadinessMsg{
			Readiness: int(readiness),
			Message:   msg,
			Endpoint:  "", // Will be set by caller if needed
			Gen:       gen,
		})
	}()

	return gen
}

// Init initializes the TUI.
func (a App) Init() tea.Cmd {
	return tea.Batch(
		a.homeModel.Init(),
		a.chatModel.Init(),
		a.sessionsModel.Init(),
		a.settingsModel.Init(),
		// Start listening for external messages
		a.listenForMessages(),
	)
}

// listenForMessages creates a command that listens for external messages.
func (a App) listenForMessages() tea.Cmd {
	return func() tea.Msg {
		return <-a.msgChan
	}
}

// Update handles messages and user input.
func ShortenModelName(model string) string {
	if model == "" {
		return "(use /model)"
	}

	// FIX: Handle numeric-only model names (user entered wrong value)
	// Return the full model name with a warning indicator
	if _, err := strconv.Atoi(model); err == nil {
		return "(invalid: " + model + ")"
	}

	tag := ""
	if idx := strings.LastIndex(model, ":"); idx != -1 {
		tag = model[idx+1:]
		model = model[:idx]
	}

	parts := strings.SplitN(model, "/", 2)
	if len(parts) == 2 {
		provider := parts[0]
		rest := parts[1]
		segments := strings.Split(rest, "-")

		short := ""
		for i := len(segments) - 1; i >= 0; i-- {
			s := segments[i]
			if strings.ContainsAny(s, "0123456789") {
				// Prefer segments that end with 'b' (like "120b" for billion parameters)
				// and are longer than current short (indicating more specific version)
				if len(s) > len(short) || (len(s) == len(short) && strings.HasSuffix(s, "b")) {
					short = s
				}
			}
		}
		if short == "" {
			short = segments[len(segments)-1]
		}

		result := provider + "..." + short
		if tag != "" {
			result += "(" + tag + ")"
		}
		return result
	}

	if len(model) > 20 {
		return model[:17] + "..."
	}
	return model
}

// ---------------------------------------------------------------------------
// StatusMsg for status updates
// ---------------------------------------------------------------------------

type StatusMsg struct {
	Text string
	Type string
}

// ---------------------------------------------------------------------------
// Run starts the TUI application.
// ---------------------------------------------------------------------------

// Run starts the TUI application and returns when it exits.
func Run(app *App) error {
	// Use AltScreen for proper TUI experience (like lumina-bot)
	p := tea.NewProgram(app, tea.WithAltScreen(),
		tea.WithInput(newOSCStrippingReader(os.Stdin)))
	_, err := p.Run()
	return err
}
