// Chat view with rich message display and input handling

package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Message types for async communication
// ---------------------------------------------------------------------------

// UserSubmitMsg is sent when user submits a message (non-blocking)
type UserSubmitMsg struct {
	Text string
}

// UserCommandMsg is sent when user enters a slash command
type UserCommandMsg struct {
	Command string
}

// ---------------------------------------------------------------------------
// ChatDelegate handles chat actions
// ---------------------------------------------------------------------------
type ChatDelegate interface {
	OnSubmit(text string) tea.Cmd
	OnCommand(command string)
}

// ---------------------------------------------------------------------------
// ChatMessage represents a message in the chat
// ---------------------------------------------------------------------------
type ChatMessage struct {
	Role      string
	Content   string
	Timestamp time.Time
	IsTool    bool
	ToolName  string
}

// ---------------------------------------------------------------------------
// ChatModel is the chat view model
// ---------------------------------------------------------------------------
type ChatModel struct {
	width    int
	height   int
	messages []ChatMessage
	viewport viewport.Model
	textarea textarea.Model
	focused  bool

	// State
	thinking     bool
	thinkingText string
	model        string
	
	// Streaming state
	streaming     bool
	streamBuffer  string
	currentTool   *ToolUseBlock

	// Delegate
	delegate ChatDelegate
}

// ToolUseBlock represents an active tool invocation
type ToolUseBlock struct {
	ID   string
	Name string
}

// NewChatModel creates a new chat model.
func NewChatModel() ChatModel {
	ta := textarea.New()
	ta.SetHeight(3)
	ta.SetWidth(80)
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.Placeholder = "Type a message..."
	ta.Focus()

	vp := viewport.New(80, 20)

	return ChatModel{
		textarea: ta,
		viewport: vp,
		messages: make([]ChatMessage, 0),
		focused:  true,
	}
}

// SetDelegate sets the chat delegate.
func (m *ChatModel) SetDelegate(delegate ChatDelegate) {
	m.delegate = delegate
}

// SetModel sets the model name.
func (m *ChatModel) SetModel(model string) {
	m.model = model
}

// GetModel returns the model name.
func (m ChatModel) GetModel() string {
	return m.model
}

// Init initializes the chat model.
func (m ChatModel) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
	)
}

// Update handles messages.
func (m ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Reserve space for input area (4 lines)
		inputHeight := 4
		vpHeight := msg.Height - inputHeight
		if vpHeight < 5 {
			vpHeight = 5
		}

		m.viewport.Width = msg.Width
		m.viewport.Height = vpHeight
		m.textarea.SetWidth(msg.Width - 4)

		m.refreshViewport()

	case tea.KeyMsg:
		if !m.focused {
			return m, nil
		}

		switch msg.Type {
		case tea.KeyEnter:
			if msg.Alt {
				// Multi-line input
				m.textarea.InsertString("\n")
				return m, nil
			}

			input := m.textarea.Value()
			if input == "" {
				return m, nil
			}

			// Handle slash commands
			trimmed := strings.TrimSpace(input)
			if strings.HasPrefix(trimmed, "/") {
				if m.delegate != nil {
					m.delegate.OnCommand(trimmed)
				}
			} else {
				// Regular message
				m.AddMessage("user", input)
				if m.delegate != nil {
					cmd := m.delegate.OnSubmit(input)
					m.textarea.SetValue("")
					m.refreshViewport()
					return m, cmd
				}
			}

			m.textarea.SetValue("")
			m.refreshViewport()
			return m, nil

		case tea.KeyCtrlC:
			// Allow Ctrl+C to propagate for quit
			return m, nil
		}

		// Update textarea
		newTA, cmd := m.textarea.Update(msg)
		m.textarea = newTA
		cmds = append(cmds, cmd)

	// -------------------------------------------------------------------------
	// Async agent messages - real-time streaming
	// -------------------------------------------------------------------------
	case AgentStartMsg:
		m.thinking = true
		m.thinkingText = "Thinking..."
		m.streaming = true
		m.streamBuffer = ""
		return m, nil

	case AgentChunkMsg:
		if m.streaming {
			m.streamBuffer += msg.Text
			// Update or create the streaming assistant message
			m.updateOrCreateStreamingMessage(m.streamBuffer)
		}
		return m, nil

	case AgentToolStartMsg:
		m.currentTool = &ToolUseBlock{ID: msg.ToolID, Name: msg.ToolName}
		m.AddToolMessage(msg.ToolName, fmt.Sprintf("Using %s...", msg.ToolName))
		return m, nil

	case AgentToolDoneMsg:
		m.currentTool = nil
		return m, nil

	case AgentDoneMsg:
		m.thinking = false
		m.streaming = false
		// Finalize the streaming message
		if m.streamBuffer != "" {
			m.finalizeStreamingMessage(m.streamBuffer)
		}
		m.streamBuffer = ""
		return m, nil

	case AgentErrorMsg:
		m.thinking = false
		m.streaming = false
		m.AddMessage("system", fmt.Sprintf("Error: %v", msg.Error))
		m.streamBuffer = ""
		return m, nil
	}

	// Update viewport for all other message types
	newVP, cmd := m.viewport.Update(msg)
	m.viewport = newVP
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the chat.
func (m ChatModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "  Initializing chat..."
	}

	// Calculate heights with minimums
	inputHeight := 3
	if m.thinking {
		inputHeight = 4
	}
	
	// Ensure minimum height for viewport
	vpHeight := m.height - inputHeight - 1 // -1 for separator
	if vpHeight < 5 {
		vpHeight = 5
	}

	// Ensure viewport has correct dimensions
	m.viewport.Width = m.width
	m.viewport.Height = vpHeight

	// Build the view
	var sections []string

	// Viewport for messages
	vpContent := m.viewport.View()
	if strings.TrimSpace(vpContent) == "" {
		vpContent = HelpDimStyle.Render("  No messages yet. Start chatting!")
	}
	
	// Constrain viewport to calculated height
	vpRendered := lipgloss.NewStyle().
		Height(vpHeight).
		MaxHeight(vpHeight).
		Render(vpContent)
	sections = append(sections, vpRendered)

	// Separator line
	sep := SeparatorStyle.Render(strings.Repeat("─", m.width))
	sections = append(sections, sep)

	// Input area (fixed height)
	var inputSection string
	prompt := PromptStyle.Render("◆ ")
	if m.thinking {
		inputSection = prompt + m.textarea.View() + "\n" + SpinnerRender(m.thinkingText)
	} else {
		inputSection = prompt + m.textarea.View()
	}
	sections = append(sections, inputSection)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// Focus focuses the chat input.
func (m *ChatModel) Focus() {
	m.focused = true
	m.textarea.Focus()
}

// Blur blurs the chat input.
func (m *ChatModel) Blur() {
	m.focused = false
	m.textarea.Blur()
}

// AddMessage adds a message to the chat.
func (m *ChatModel) AddMessage(role, content string) {
	msg := ChatMessage{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}
	m.messages = append(m.messages, msg)
	m.refreshViewport()
}

// AddToolMessage adds a tool message to the chat.
func (m *ChatModel) AddToolMessage(toolName, content string) {
	msg := ChatMessage{
		Role:      "tool",
		Content:   content,
		Timestamp: time.Now(),
		IsTool:    true,
		ToolName:  toolName,
	}
	m.messages = append(m.messages, msg)
	m.refreshViewport()
}

// SetInput sets the input text.
func (m *ChatModel) SetInput(text string) {
	m.textarea.SetValue(text)
}

// GetInput returns the input text.
func (m ChatModel) GetInput() string {
	return m.textarea.Value()
}

// ClearInput clears the input.
func (m *ChatModel) ClearInput() {
	m.textarea.SetValue("")
}

// SetThinking sets the thinking state.
func (m *ChatModel) SetThinking(thinking bool, text string) {
	m.thinking = thinking
	m.thinkingText = text
	if text == "" {
		m.thinkingText = "Thinking..."
	}
}

// ConsumesTab returns whether this view consumes Tab key.
func (m ChatModel) ConsumesTab() bool {
	return false
}

// ConsumesEsc returns whether this view consumes Esc key.
func (m ChatModel) ConsumesEsc() bool {
	return false
}

// Scroll scrolls the viewport.
func (m *ChatModel) Scroll(lines int) {
	if lines > 0 {
		m.viewport.LineDown(lines)
	} else {
		m.viewport.LineUp(-lines)
	}
}

// GotoTop scrolls to top.
func (m *ChatModel) GotoTop() {
	m.viewport.GotoTop()
}

// GotoBottom scrolls to bottom.
func (m *ChatModel) GotoBottom() {
	m.viewport.GotoBottom()
}

// updateOrCreateStreamingMessage updates the last assistant message or creates one
func (m *ChatModel) updateOrCreateStreamingMessage(content string) {
	// Check if the last message is an assistant message we're streaming into
	if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == "assistant" {
		// Update existing streaming message
		m.messages[len(m.messages)-1].Content = content
	} else {
		// Create new streaming message
		m.messages = append(m.messages, ChatMessage{
			Role:      "assistant",
			Content:   content,
			Timestamp: time.Now(),
		})
	}
	m.refreshViewport()
}

// finalizeStreamingMessage finalizes the streaming message
func (m *ChatModel) finalizeStreamingMessage(content string) {
	if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == "assistant" {
		m.messages[len(m.messages)-1].Content = content
		m.messages[len(m.messages)-1].Timestamp = time.Now()
	}
	m.refreshViewport()
}

// refreshViewport refreshes the viewport content.
func (m *ChatModel) refreshViewport() {
	var content strings.Builder

	for _, msg := range m.messages {
		content.WriteString(m.renderMessage(msg))
		content.WriteString("\n\n")
	}

	m.viewport.SetContent(content.String())
	m.viewport.GotoBottom()
}

func (m ChatModel) renderMessage(msg ChatMessage) string {
	switch msg.Role {
	case "user":
		return m.renderUserMessage(msg)
	case "assistant":
		return m.renderAssistantMessage(msg)
	case "tool":
		return m.renderToolMessage(msg)
	case "system":
		return m.renderSystemMessage(msg)
	default:
		return msg.Content
	}
}

func (m ChatModel) renderUserMessage(msg ChatMessage) string {
	var b strings.Builder

	// Header
	header := UserPromptStyle.Render("You")
	if !msg.Timestamp.IsZero() {
		header += TimestampStyle.Render(" " + msg.Timestamp.Format("15:04"))
	}
	b.WriteString(header)
	b.WriteString("\n")

	// Content with left border
	content := MessageBubbleUser.Width(m.width - 4).Render(msg.Content)
	b.WriteString(content)

	return b.String()
}

func (m ChatModel) renderAssistantMessage(msg ChatMessage) string {
	var b strings.Builder

	// Header
	header := AssistantStyle.Render("Agent")
	if !msg.Timestamp.IsZero() {
		header += TimestampStyle.Render(" " + msg.Timestamp.Format("15:04"))
	}
	b.WriteString(header)
	b.WriteString("\n")

	// Content with left border
	content := MessageBubbleAssistant.Width(m.width - 4).Render(msg.Content)
	b.WriteString(content)

	return b.String()
}

func (m ChatModel) renderToolMessage(msg ChatMessage) string {
	var b strings.Builder

	// Tool header
	toolHeader := ToolCallStyle.Render(fmt.Sprintf("[%s]", msg.ToolName))
	b.WriteString(toolHeader)
	b.WriteString("\n")

	// Content
	b.WriteString(HelpDimStyle.Render(Truncate(msg.Content, m.width-4)))

	return b.String()
}

func (m ChatModel) renderSystemMessage(msg ChatMessage) string {
	return SystemMessageStyle.Render(msg.Content)
}
