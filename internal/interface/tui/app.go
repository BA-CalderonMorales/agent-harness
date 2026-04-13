// Main TUI application with tab-based navigation
// Inspired by lumina-bot's exceptional TUI experience

package tui

import (
	"context"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// View identifiers
// ---------------------------------------------------------------------------
type viewID int

const (
	viewChat viewID = iota
	viewSessions
	viewSettings
	viewCount
)

var viewLabels = [viewCount]string{
	"Chat", "Sessions", "Settings",
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
	chatModel      ChatModel
	sessionsModel  SessionsModel
	settingsModel  SettingsModel
	approvalDialog ApprovalDialogModel

	// UI state
	showHelp       bool
	helpModel      Help
	commandPalette CommandPaletteModel
	modelPicker    ModelPickerModel
	tabActivity    [viewCount]bool

	// Status
	statusMessage string
	statusType    string // "info", "success", "error", "warning"

	// External message channel for async updates
	msgChan chan tea.Msg

	// Handlers for user actions (set by main.go)
	onUserSubmit  func(string, App)
	onUserCommand func(string, App)

	// Agent cancellation context
	agentCancelFunc context.CancelFunc
}

// NewApp creates the root app model.
func NewApp() *App {
	return &App{
		activeView:     viewChat,
		mode:           ModeInsert,
		chatModel:      NewChatModel(),
		sessionsModel:  NewSessionsModel(),
		settingsModel:  NewSettingsModel(),
		approvalDialog: NewApprovalDialog(),
		helpModel:      NewHelp(),
		commandPalette: NewCommandPalette(),
		modelPicker:    NewModelPicker(),
		msgChan:        make(chan tea.Msg, 64),
	}
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
func (a *App) SetUserSubmitHandler(handler func(string, App)) {
	a.onUserSubmit = handler
}

// SetUserCommandHandler sets the handler for slash commands.
func (a *App) SetUserCommandHandler(handler func(string, App)) {
	a.onUserCommand = handler
}

// SetSessionsDelegate sets the sessions handler delegate.
func (a *App) SetSessionsDelegate(delegate SessionsDelegate) {
	a.sessionsModel.SetDelegate(delegate)
}

// SetSettingsDelegate sets the settings handler delegate.
func (a *App) SetSettingsDelegate(delegate SettingsDelegate) {
	a.settingsModel.SetDelegate(delegate)
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
		// Channel full, drop message (shouldn't happen with buffer)
	}
}

// Init initializes the TUI.
func (a App) Init() tea.Cmd {
	return tea.Batch(
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
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	// -------------------------------------------------------------------------
	// Global keys
	// -------------------------------------------------------------------------
	case tea.KeyMsg:
		// Help toggle — only in normal mode to avoid interfering with typing
		if msg.String() == "?" && !a.showHelp && a.mode == ModeNormal {
			a.showHelp = true
			a.helpModel.Open(a.width, a.height, "")
			return a, nil
		}

		// When help is open, delegate scrolling to the help viewport
		if a.showHelp {
			switch msg.String() {
			case "?", "esc", "q":
				a.showHelp = false
				return a, nil
			}
			return a, nil
		}

		// When command palette is open, delegate to it
		if a.commandPalette.IsShowing() {
			closed, cmd := a.commandPalette.Update(msg)
			if closed {
				if selected := a.commandPalette.SelectedCommand(); selected != nil {
					return a.handlePaletteSelection(selected)
				}
			}
			return a, cmd
		}

		// When model picker is open, delegate to it
		if a.modelPicker.IsShowing() {
			closed, cmd := a.modelPicker.Update(msg)
			if closed {
				if selected := a.modelPicker.SelectedModel(); selected != nil {
					cmdText := "/model " + selected.ID
					if a.onUserCommand != nil {
						go a.onUserCommand(cmdText, a)
					}
				}
			}
			return a, cmd
		}

		// When approval dialog is open, delegate to it
		if a.approvalDialog.IsVisible() {
			dialog, cmd := a.approvalDialog.Update(msg)
			a.approvalDialog = dialog
			return a, cmd
		}

		switch msg.Type {
		case tea.KeyCtrlC:
			return a, tea.Quit

		case tea.KeyTab:
			if !a.activeViewConsumesTab() {
				a.blurActive()
				a.activeView = (a.activeView + 1) % viewCount
				a.focusActive()
				return a, a.initActiveView()
			}

		case tea.KeyShiftTab:
			if !a.activeViewConsumesTab() {
				a.blurActive()
				a.activeView = (a.activeView - 1 + viewCount) % viewCount
				a.focusActive()
				return a, a.initActiveView()
			}

		case tea.KeyEsc:
			// If agent is running, cancel it first
			if a.agentCancelFunc != nil {
				a.CancelAgent()
				return a, func() tea.Msg {
					return AgentCancelMsg{}
				}
			}
			if !a.activeViewConsumesEsc() {
				a.mode = ModeNormal
				a.blurActive()
				return a, nil
			}
		}

		// Mode switching
		switch msg.String() {
		case "ctrl+n":
			a.mode = ModeNormal
			a.blurActive()
			return a, nil
		case "i":
			if a.mode == ModeNormal {
				a.mode = ModeInsert
				a.focusActive()
				return a, nil
			}
		}

		// View switching shortcuts
		switch msg.String() {
		case "ctrl+1", "1":
			return a, a.switchView(viewChat)
		case "ctrl+2", "2":
			return a, a.switchView(viewSessions)
		case "ctrl+3", "3":
			return a, a.switchView(viewSettings)
		}

		// Navigation in normal mode
		if a.mode == ModeNormal {
			switch msg.String() {
			case "j", "down":
				a.scrollActiveView(1)
				return a, nil
			case "k", "up":
				a.scrollActiveView(-1)
				return a, nil
			case "g", "home":
				a.gotoActiveViewTop()
				return a, nil
			case "G", "end":
				a.gotoActiveViewBottom()
				return a, nil
			}
		}

	// -------------------------------------------------------------------------
	// Window resize
	// -------------------------------------------------------------------------
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height

		// Reserve space for tab bar (2) + status bar (1)
		reserved := 3
		contentMsg := tea.WindowSizeMsg{
			Width:  msg.Width,
			Height: msg.Height - reserved,
		}

		// Propagate to sub-models
		chatModel, cmd := a.chatModel.Update(contentMsg)
		a.chatModel = chatModel.(ChatModel)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

		sessionsModel, cmd := a.sessionsModel.Update(contentMsg)
		a.sessionsModel = sessionsModel.(SessionsModel)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

		settingsModel, cmd := a.settingsModel.Update(contentMsg)
		a.settingsModel = settingsModel.(SettingsModel)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

		return a, tea.Batch(cmds...)

	// -------------------------------------------------------------------------
	// Status messages
	// -------------------------------------------------------------------------
	case StatusMsg:
		a.statusMessage = msg.Text
		a.statusType = msg.Type
		// Continue listening for more messages
		cmds = append(cmds, a.listenForMessages())
		// Return early - status is handled at app level
		return a, tea.Batch(cmds...)

	// -------------------------------------------------------------------------
	// User submission (non-blocking) - spawn handler in goroutine
	// -------------------------------------------------------------------------
	case UserSubmitMsg:
		if a.onUserSubmit != nil {
			// Spawn handler in goroutine to avoid blocking Update loop
			go a.onUserSubmit(msg.Text, a)
		}

	// -------------------------------------------------------------------------
	// User command (non-blocking) - spawn handler in goroutine
	// -------------------------------------------------------------------------
	case UserCommandMsg:
		if a.onUserCommand != nil {
			// Spawn handler in goroutine to avoid blocking Update loop
			go a.onUserCommand(msg.Command, a)
		}

	// -------------------------------------------------------------------------
	// Streaming messages from agent loop - forward to chat
	// These are handled HERE ONLY to avoid double-processing
	// -------------------------------------------------------------------------
	case StreamStartMsg, StreamChunkMsg, StreamMessageMsg, StreamErrorMsg, StreamDoneMsg,
		AgentStartMsg, AgentChunkMsg, AgentToolStartMsg, AgentToolDoneMsg, AgentDoneMsg, AgentErrorMsg, AgentConnectingMsg:
		chatModel, cmd := a.chatModel.Update(msg)
		a.chatModel = chatModel.(ChatModel)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		// Continue listening for more messages
		cmds = append(cmds, a.listenForMessages())
		// Return early - do NOT delegate to active view (would cause duplicates)
		return a, tea.Batch(cmds...)

	// -------------------------------------------------------------------------
	// Command palette open request
	// -------------------------------------------------------------------------
	case openCommandPaletteMsg:
		a.commandPalette.Open(a.width, a.height)
		return a, nil

	// -------------------------------------------------------------------------
	// Model picker open request
	// -------------------------------------------------------------------------
	case openModelPickerMsg:
		a.modelPicker.Open(a.width, a.height)
		return a, nil

	// -------------------------------------------------------------------------
	// Quit request
	// -------------------------------------------------------------------------
	case QuitMsg:
		return a, tea.Quit

	// -------------------------------------------------------------------------
	// Clear chat request - handle globally so it works from any view
	// -------------------------------------------------------------------------
	case ClearChatMsg:
		chatModel, cmd := a.chatModel.Update(msg)
		a.chatModel = chatModel.(ChatModel)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return a, tea.Batch(cmds...)

	// -------------------------------------------------------------------------
	// Approval request - show the approval dialog
	// -------------------------------------------------------------------------
	case ApprovalRequestMsg:
		a.approvalDialog.Show(msg.Request)
		return a, nil

	// -------------------------------------------------------------------------
	// Tool executing notification - show in chat
	// -------------------------------------------------------------------------
	case ToolExecutingMsg:
		// Add or update tool message with running status
		a.chatModel.AddOrUpdateToolMessage(msg.ToolID, msg.ToolName, getToolDisplayName(msg.ToolName),
			msg.Command, ToolStatusRunning)
		return a, nil

	// -------------------------------------------------------------------------
	// Agent cancellation - handle cancel signal
	// -------------------------------------------------------------------------
	case AgentCancelMsg:
		a.chatModel.AddMessage("system", "Agent execution cancelled by user (ESC)")
		return a, nil
	}

	// -------------------------------------------------------------------------
	// Delegate to active view (non-streaming messages only)
	// -------------------------------------------------------------------------
	var cmd tea.Cmd
	switch a.activeView {
	case viewChat:
		model, c := a.chatModel.Update(msg)
		a.chatModel = model.(ChatModel)
		cmd = c
	case viewSessions:
		model, c := a.sessionsModel.Update(msg)
		a.sessionsModel = model.(SessionsModel)
		cmd = c
	case viewSettings:
		model, c := a.settingsModel.Update(msg)
		a.settingsModel = model.(SettingsModel)
		cmd = c
	}
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return a, tea.Batch(cmds...)
}

// View renders the TUI.
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

// renderStatusBar renders the status bar at the bottom.
// Shows meaningful model info and actionable hints - never just "default".
func (a App) renderStatusBar() string {
	var parts []string

	// Left: Status indicator based on state
	status := StatusOnline.Render("[ready]")
	modelName := a.chatModel.GetModel()

	// If no model configured, show warning indicator
	if modelName == "" {
		status = StatusConnecting.Render("[! no model]")
	}
	parts = append(parts, " "+status+" Agent Harness")

	// Middle: Model info - never shows "default"
	model := ShortenModelName(modelName)
	modelDisplay := StatusLabel.Render("model:" + model)
	// Highlight if no model set
	if modelName == "" {
		modelDisplay = WarningStyle.Render("model:" + model)
	}
	parts = append(parts, modelDisplay)

	// Right: Help hints and mode
	modeStr := "[typing]"
	if a.mode == ModeNormal {
		modeStr = "[navigate]"
	}
	if a.mode == ModeNormal {
		right := StatusHintStyle.Render("Tab: views  ?: help  Ctrl+C: quit  ") + WarningStyle.Render(modeStr)
		parts = append(parts, right)
	} else {
		right := StatusHintStyle.Render("Tab: views  ?: help  Ctrl+C: quit  ") + StatusHintStyle.Render(modeStr)
		parts = append(parts, right)
	}

	// Join with flexible spacing
	content := strings.Join(parts, "  ")

	// If too wide for terminal, use minimal version but keep model info meaningful
	if lipgloss.Width(content) > a.width-4 {
		shortModel := ShortenModelName(modelName)
		if modelName == "" {
			content = " " + status + "  " + shortModel
		} else {
			content = " " + status + "  " + shortModel + "  Ctrl+C: quit"
		}
	}

	return StatusBarStyle.Width(a.width).PaddingBottom(1).PaddingLeft(1).Render(content)
}

// ---------------------------------------------------------------------------
// View switching helpers
// ---------------------------------------------------------------------------

func (a *App) switchView(v viewID) tea.Cmd {
	a.blurActive()
	a.activeView = v
	a.focusActive()
	return a.initActiveView()
}

func (a *App) focusActive() {
	a.tabActivity[a.activeView] = false
	switch a.activeView {
	case viewChat:
		a.chatModel.Focus()
	case viewSessions:
		a.sessionsModel.Focus()
	case viewSettings:
		a.settingsModel.Focus()
	}
}

func (a *App) blurActive() {
	switch a.activeView {
	case viewChat:
		a.chatModel.Blur()
	case viewSessions:
		a.sessionsModel.Blur()
	case viewSettings:
		a.settingsModel.Blur()
	}
}

func (a *App) initActiveView() tea.Cmd {
	switch a.activeView {
	case viewChat:
		return a.chatModel.Init()
	case viewSessions:
		return a.sessionsModel.Init()
	case viewSettings:
		return a.settingsModel.Init()
	}
	return nil
}

func (a *App) activeViewConsumesTab() bool {
	switch a.activeView {
	case viewChat:
		return a.chatModel.ConsumesTab()
	case viewSessions:
		return a.sessionsModel.ConsumesTab()
	case viewSettings:
		return a.settingsModel.ConsumesTab()
	}
	return false
}

func (a *App) activeViewConsumesEsc() bool {
	switch a.activeView {
	case viewChat:
		return a.chatModel.ConsumesEsc()
	case viewSessions:
		return a.sessionsModel.ConsumesEsc()
	case viewSettings:
		return a.settingsModel.ConsumesEsc()
	}
	return false
}

func (a *App) scrollActiveView(lines int) {
	switch a.activeView {
	case viewChat:
		a.chatModel.Scroll(lines)
	case viewSessions:
		a.sessionsModel.Scroll(lines)
	case viewSettings:
		a.settingsModel.Scroll(lines)
	}
}

func (a *App) gotoActiveViewTop() {
	switch a.activeView {
	case viewChat:
		a.chatModel.GotoTop()
	case viewSessions:
		a.sessionsModel.GotoTop()
	case viewSettings:
		a.settingsModel.GotoTop()
	}
}

func (a *App) gotoActiveViewBottom() {
	switch a.activeView {
	case viewChat:
		a.chatModel.GotoBottom()
	case viewSessions:
		a.sessionsModel.GotoBottom()
	case viewSettings:
		a.settingsModel.GotoBottom()
	}
}

// ---------------------------------------------------------------------------
// Public API for external interaction
// ---------------------------------------------------------------------------

// AddMessage adds a message to the chat.
func (a *App) AddMessage(role, content string) {
	a.chatModel.AddMessage(role, content)
}

// SetInput sets the chat input text.
func (a *App) SetInput(text string) {
	a.chatModel.SetInput(text)
}

// GetInput returns the current chat input.
func (a *App) GetInput() string {
	return a.chatModel.GetInput()
}

// ClearInput clears the chat input.
func (a *App) ClearInput() {
	a.chatModel.ClearInput()
}

// SetThinking sets the thinking state.
func (a *App) SetThinking(thinking bool, text string) {
	a.chatModel.SetThinking(thinking, text)
}

// ShowStatus shows a status message.
func (a *App) ShowStatus(text string, statusType string) {
	a.statusMessage = text
	a.statusType = statusType
}

// RefreshSessions refreshes the sessions list.
func (a *App) RefreshSessions(sessions []SessionInfo) {
	a.sessionsModel.SetSessions(sessions)
}

// SetSettings sets the settings list.
func (a *App) SetSettings(settings []Setting) {
	a.settingsModel.SetSettings(settings)
}

// SetModels sets the available models for the model picker.
func (a *App) SetModels(models []ModelItem) {
	a.modelPicker.SetModels(models)
}

// SetChatModel sets the current model name for display in the status bar.
func (a *App) SetChatModel(model string) {
	a.chatModel.SetModel(model)
}

// handlePaletteSelection handles a command selected from the palette.
// Commands with no arguments are executed immediately.
// /model with no args opens the model picker.
// Everything else is inserted into the input with a trailing space.
func (a *App) handlePaletteSelection(selected *commandInfo) (App, tea.Cmd) {
	noArgCommands := map[string]bool{
		"/help":          true,
		"/status":        true,
		"/clear":         true,
		"/compact":       true,
		"/cost":          true,
		"/diff":          true,
		"/version":       true,
		"/config":        true,
		"/workspace":     true,
		"/quit":          true,
		"/current-model": true,
		"/reset":         true,
		"/agents":        true,
		"/skills":        true,
	}

	cmdName := selected.Command
	if noArgCommands[cmdName] {
		if a.onUserCommand != nil {
			a.chatModel.AddMessage("user", cmdName)
			a.onUserCommand(cmdName, *a)
		}
		return *a, nil
	}

	if cmdName == "/model" && selected.Args == "" {
		a.modelPicker.Open(a.width, a.height)
		return *a, nil
	}

	a.chatModel.SetInput(cmdName + " ")
	return *a, nil
}

// ShortenModelName returns a compact display name for a model.
// Never returns "default" - always shows something actionable or informative.
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
	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
