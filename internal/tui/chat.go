// Chat view with rich message display and input handling
// Inspired by lumina-bot's streaming integration

package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"github.com/charmbracelet/bubbles/cursor"
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

	// Streaming state
	streaming    bool
	streamText   string
	streamStart  time.Time
	streamChunks int

	// Model info
	model string
}

// NewChatModel creates a new chat model.
func NewChatModel() ChatModel {
	ta := textarea.New()
	ta.SetHeight(3)
	ta.SetWidth(80)
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.Placeholder = "Type a message... (/help for commands)"
	ta.Focus()

	// Steady cursor - no blink for better performance in Termux
	ta.Cursor.SetMode(cursor.CursorStatic)
	ta.Cursor.Style = lipgloss.NewStyle().Background(ColorPrimary)
	ta.Cursor.TextStyle = lipgloss.NewStyle().Background(ColorPrimary).Foreground(ColorSurface)

	vp := viewport.New(80, 20)

	return ChatModel{
		textarea: ta,
		viewport: vp,
		messages: make([]ChatMessage, 0),
		focused:  true,
	}
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
	return textarea.Blink
}

// Update handles messages.
func (m ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalcLayout()

	case tea.KeyMsg:
		if m.streaming {
			// Allow Ctrl+C to cancel streaming
			if msg.Type == tea.KeyCtrlC {
				return m, nil
			}
			// Block other input while streaming
			return m, nil
		}

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

			input := strings.TrimSpace(m.textarea.Value())
			if input == "" {
				return m, nil
			}

			// Clear input immediately
			m.textarea.SetValue("")
			m.textarea.SetHeight(3)

			// Add user message locally
			m.AddMessage("user", input)

			// Return command for async processing (non-blocking)
			if strings.HasPrefix(input, "/") {
				return m, func() tea.Msg {
					return UserCommandMsg{Command: input}
				}
			}
			return m, func() tea.Msg {
				return UserSubmitMsg{Text: input}
			}

		case tea.KeyCtrlC:
			// Allow Ctrl+C to propagate for quit
			return m, nil
		}

		// Update textarea for normal input
		newTA, cmd := m.textarea.Update(msg)
		m.textarea = newTA
		cmds = append(cmds, cmd)

	// -------------------------------------------------------------------------
	// Streaming messages from agent loop
	// -------------------------------------------------------------------------
	case StreamStartMsg:
		m.streaming = true
		m.streamText = ""
		m.streamChunks = 0
		m.streamStart = time.Now()
		m.refreshViewport()

	case StreamChunkMsg:
		m.streamText += msg.Text
		m.streamChunks++
		m.refreshViewport()

	case StreamMessageMsg:
		// Complete message received - add it to the chat
		for _, block := range msg.Message.Content {
			if textBlock, ok := block.(types.TextBlock); ok {
				m.AddMessage("assistant", textBlock.Text)
			}
		}

	case StreamErrorMsg:
		m.AddMessage("system", fmt.Sprintf("Error: %s", msg.Error))
		m.streaming = false
		m.streamText = ""
		m.refreshViewport()

	case StreamDoneMsg:
		m.streaming = false
		m.streamText = ""
		m.refreshViewport()
	}

	// Update viewport
	newVP, cmd := m.viewport.Update(msg)
	m.viewport = newVP
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// recalcLayout recalculates layout based on current dimensions
func (m *ChatModel) recalcLayout() {
	inputHeight := 4 // textarea + border + hint line
	vpHeight := m.height - inputHeight
	if vpHeight < 5 {
		vpHeight = 5
	}

	m.viewport.Width = m.width
	m.viewport.Height = vpHeight
	m.textarea.SetWidth(m.width - 4)

	m.refreshViewport()
}

// View renders the chat.
func (m ChatModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "  Initializing chat..."
	}

	// Calculate heights
	inputHeight := 3
	if m.streaming {
		inputHeight = 4
	}

	vpHeight := m.height - inputHeight - 1
	if vpHeight < 5 {
		vpHeight = 5
	}

	m.viewport.Width = m.width
	m.viewport.Height = vpHeight

	// Build the view
	var sections []string

	// Viewport for messages
	sections = append(sections, m.viewport.View())

	// Separator line
	sep := SeparatorStyle.Render(strings.Repeat("─", m.width))
	sections = append(sections, sep)

	// Input area
	var inputSection string
	prompt := PromptStyle.Render("◆ ")
	if m.streaming {
		elapsed := time.Since(m.streamStart).Seconds()
		status := fmt.Sprintf(" thinking... (%.1fs, %d chunks)", elapsed, m.streamChunks)
		inputSection = prompt + m.textarea.View() + "\n" + SpinnerStyle.Render(status)
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
	m.streaming = thinking
	if thinking {
		m.streamStart = time.Now()
		m.streamChunks = 0
		m.streamText = ""
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

// refreshViewport refreshes the viewport content.
func (m *ChatModel) refreshViewport() {
	var content strings.Builder

	for _, msg := range m.messages {
		content.WriteString(m.renderMessage(msg))
		content.WriteString("\n\n")
	}

	// Show streaming content if active
	if m.streaming && m.streamText != "" {
		content.WriteString(m.renderStreamingMessage())
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
	header := UserPromptStyle.Render("  You")
	if !msg.Timestamp.IsZero() {
		header += TimestampStyle.Render(" " + msg.Timestamp.Format("15:04"))
	}
	b.WriteString(header)
	b.WriteString("\n")

	// Content
	content := MessageBubbleUser.Width(m.width - 4).Render(msg.Content)
	b.WriteString(content)

	return b.String()
}

func (m ChatModel) renderAssistantMessage(msg ChatMessage) string {
	var b strings.Builder

	// Header
	header := AssistantStyle.Render("  Agent")
	if !msg.Timestamp.IsZero() {
		header += TimestampStyle.Render(" " + msg.Timestamp.Format("15:04"))
	}
	b.WriteString(header)
	b.WriteString("\n")

	// Content
	content := MessageBubbleAssistant.Width(m.width - 4).Render(msg.Content)
	b.WriteString(content)

	return b.String()
}

func (m ChatModel) renderToolMessage(msg ChatMessage) string {
	var b strings.Builder

	// Tool header
	toolHeader := ToolCallStyle.Render(fmt.Sprintf("  [%s]", msg.ToolName))
	b.WriteString(toolHeader)
	b.WriteString("\n")

	// Content
	content := HelpDimStyle.Render(Truncate(msg.Content, m.width-6))
	b.WriteString("  " + content)

	return b.String()
}

func (m ChatModel) renderSystemMessage(msg ChatMessage) string {
	return SystemMessageStyle.Width(m.width - 4).Render(msg.Content)
}

func (m ChatModel) renderStreamingMessage() string {
	var b strings.Builder

	// Header with spinner
	elapsed := time.Since(m.streamStart).Seconds()
	header := AssistantStyle.Render("  Agent") +
		StreamingStyle.Render(fmt.Sprintf("  thinking... (%.1fs)", elapsed))
	b.WriteString(header)
	b.WriteString("\n")

	// Streaming content
	content := MessageBubbleAssistant.Width(m.width - 4).Render(m.streamText + "▌")
	b.WriteString(content)

	return b.String()
}
