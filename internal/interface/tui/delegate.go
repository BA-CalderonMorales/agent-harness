// Async delegation pattern for TUI
// Prevents blocking the Bubble Tea event loop during agent processing

package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// SubmitMsg is sent when the user submits a message
type SubmitMsg struct {
	Text string
}

// CommandMsg is sent when the user executes a slash command
type CommandMsg struct {
	Command string
}

// AsyncChatModel wraps ChatModel with async submit handling
type AsyncChatModel struct {
	ChatModel
	onSubmit  func(string) tea.Cmd
	onCommand func(string) tea.Cmd
}

// NewAsyncChatModel creates a chat model that returns commands instead of blocking
func NewAsyncChatModel(onSubmit func(string) tea.Cmd, onCommand func(string) tea.Cmd) AsyncChatModel {
	return AsyncChatModel{
		ChatModel: NewChatModel(),
		onSubmit:  onSubmit,
		onCommand: onCommand,
	}
}

// Update handles messages and delegates submit as commands
func (m AsyncChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyEnter && !msg.Alt {
			input := strings.TrimSpace(m.textarea.Value())
			if input == "" {
				return m, nil
			}

			// Clear input immediately for responsiveness
			m.textarea.SetValue("")
			m.textarea.SetHeight(3)

			// Add user message to chat
			m.AddMessage("user", input)

			// Return command for async processing
			if strings.HasPrefix(input, "/") {
				if m.onCommand != nil {
					return m, m.onCommand(input)
				}
			} else {
				if m.onSubmit != nil {
					return m, m.onSubmit(input)
				}
			}
			return m, nil
		}
	}

	// Delegate to base chat model
	model, cmd := m.ChatModel.Update(msg)
	m.ChatModel = model.(ChatModel)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}
