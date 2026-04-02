package ui

import (
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"os"
)

// Styles for the TUI.
var (
	userStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7D56F4")).
		Bold(true)

	agentStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#04B575"))

	toolStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F4D03F")).
		Italic(true)

	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E74C3C"))

	helpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262"))
)

// Message represents a rendered message in the TUI.
type Message struct {
	Role    string
	Content string
	IsTool  bool
}

// Model is the bubbletea model for the agent harness TUI.
type Model struct {
	messages   []Message
	input      string
	width      int
	height     int
	sending    bool
	quitting   bool
	onSubmit   func(string) // callback when user submits input
	onQuit     func()       // callback when user quits
	isTermux   bool
	termWidth  int
}

// NewModel creates a new TUI model.
func NewModel(onSubmit func(string), onQuit func()) Model {
	// Detect Termux environment
	isTermux := os.Getenv("TERMUX_VERSION") != "" || 
		strings.Contains(os.Getenv("HOME"), "com.termux")

	return Model{
		messages:  make([]Message, 0),
		onSubmit:  onSubmit,
		onQuit:    onQuit,
		isTermux:  isTermux,
		termWidth: 80,
	}
}

// AddUserMessage adds a user message to the chat.
func (m *Model) AddUserMessage(content string) {
	m.messages = append(m.messages, Message{Role: "user", Content: content})
}

// AddAgentMessage adds an agent message to the chat.
func (m *Model) AddAgentMessage(content string) {
	m.messages = append(m.messages, Message{Role: "agent", Content: content})
}

// AddToolMessage adds a tool use message.
func (m *Model) AddToolMessage(content string) {
	m.messages = append(m.messages, Message{Role: "tool", Content: content, IsTool: true})
}

// AddErrorMessage adds an error message.
func (m *Model) AddErrorMessage(content string) {
	m.messages = append(m.messages, Message{Role: "error", Content: content})
}

// SetSending updates the sending state.
func (m *Model) SetSending(sending bool) {
	m.sending = sending
}

// Init initializes the TUI.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages and user input.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			if m.onQuit != nil {
				m.onQuit()
			}
			return m, tea.Quit

		case tea.KeyEnter:
			if m.input != "" && m.onSubmit != nil {
				content := m.input
				m.input = ""
				m.sending = true
				m.onSubmit(content)
			}

		case tea.KeyBackspace:
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}

		case tea.KeyRunes:
			// Handle actual character input - runes are printable characters
			m.input += string(msg.Runes)

		case tea.KeySpace:
			m.input += " "

		default:
			// For other keys, only add if they represent actual characters
			// and are not control characters
			if len(msg.Runes) > 0 {
				for _, r := range msg.Runes {
					if r >= 32 && r < 127 {
						m.input += string(r)
					}
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.termWidth = msg.Width
	}

	return m, nil
}

// View renders the TUI.
func (m Model) View() string {
	if m.quitting {
		return "Goodbye.\n"
	}

	var b strings.Builder

	// Render messages
	for _, msg := range m.messages {
		b.WriteString(m.renderMessage(msg))
		b.WriteString("\n\n")
	}

	if m.sending {
		b.WriteString(agentStyle.Render("Agent is thinking..."))
		b.WriteString("\n\n")
	}

	// Input prompt - use simple clean prompt for Termux
	if m.isTermux {
		b.WriteString(userStyle.Render("> "))
	} else {
		b.WriteString(userStyle.Render("> "))
	}
	b.WriteString(m.input)

	// Help line
	b.WriteString("\n")
	if m.isTermux {
		// Simpler help for mobile
		b.WriteString(helpStyle.Render("ctrl+c: quit | enter: send"))
	} else {
		b.WriteString(helpStyle.Render("esc: quit | enter: send"))
	}

	return b.String()
}

func (m Model) renderMessage(msg Message) string {
	switch msg.Role {
	case "user":
		return userStyle.Render("You: ") + msg.Content
	case "agent":
		return agentStyle.Render("Agent: ") + msg.Content
	case "tool":
		return toolStyle.Render("Tool: ") + msg.Content
	case "error":
		return errorStyle.Render("Error: ") + msg.Content
	default:
		return msg.Content
	}
}

// Run starts the bubbletea program.
func Run(initial Model) error {
	p := tea.NewProgram(initial)
	_, err := p.Run()
	return err
}
