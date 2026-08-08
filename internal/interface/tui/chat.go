// Chat view with rich message display and input handling

package tui

import (
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"strings"
	"time"
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

// submitTimerMsg is sent when the Enter-debounce timer fires.
type submitTimerMsg struct {
	generation int
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
	ID              string // Unique identifier for message replacement
	Role            string
	Content         string
	Timestamp       time.Time
	IsTool          bool
	ToolName        string
	ToolDisplayName string        // User-friendly display name for the tool
	ToolStatus      ToolStatus    // pending, running, success, error
	ResponseTime    time.Duration // Time taken to generate this response
}

// ToolStatus represents the execution state of a tool
type ToolStatus string

const (
	ToolStatusPending  ToolStatus = "pending"
	ToolStatusRunning  ToolStatus = "running"
	ToolStatusSuccess  ToolStatus = "success"
	ToolStatusError    ToolStatus = "error"
	ToolStatusComplete ToolStatus = "complete" // Generic completion (no success/error distinction)
)

// Paste detection thresholds.
const (
	PasteDisplayThreshold   = 200 // min chars to collapse a pasted message
	PasteHeuristicThreshold = 20  // min length jump in one keystroke to detect paste
	MinInputRows            = 1
	MaxInputRows            = 4
)

// Composer layout: a centered column with breathing room above and below the
// input text, plus a mode line (mode · model · provider · reasoning effort).
const (
	ComposerColumnWidth = 84 // centered max width for the composer block
	ComposerTopPadding  = 1  // blank rows above the input text
	ComposerGapRows     = 1  // blank rows between the input and the mode line
)

// SubmitDebounceDuration is the window after Enter during which another
// keystroke causes the Enter to be treated as a newline (paste continuation).
var SubmitDebounceDuration = 80 * time.Millisecond

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

	// Composer mode line metadata (set by the app-level runtime context)
	provider  string
	effort    string
	modeLabel string // "typing" / "navigate" when persona is empty

	// Streaming state
	streaming       bool
	streamBuffer    string
	currentTool     *ToolUseBlock
	turnInterrupted bool

	// Index of the assistant message currently being streamed. Tracked so that
	// mid-stream system/user messages do not break the update target, while
	// ensuring a new user turn always gets a fresh assistant message.
	currentStreamingAssistantIdx int

	// Timer state for response tracking
	startTime    time.Time
	elapsed      time.Duration
	timerRunning bool
	chunkCount   int

	// Tool animation state (for yolo mode - single animated line)
	toolAnimation *ToolAnimationState

	// Current tool message for in-place updates (replaces previous tool display)
	currentToolMsg *ChatMessage

	// completedToolMsgs tracks all finalized tool messages for the current turn.
	// Previously this was a single pointer for single-line replacement, but users
	// want to see every tool call that happens during a conversation turn.
	completedToolMsgs []ChatMessage

	// Delegate
	delegate ChatDelegate

	// Inline command suggestions (replaces modal palette)
	showSuggestions     bool
	suggestions         []string
	suggestionCursor    int
	suggestionOffset    int // scroll window start
	allCommands         []string
	commandDescriptions map[string]string

	// Persona for contextual UI behavior
	persona string

	// Paste detection state
	pasteDetected bool // true if current input was detected as a paste

	// Debounce state for distinguishing intentional Enter from pasted newlines
	pendingSubmit    bool
	pendingSubmitGen int

	// Steer queue holds user messages to be auto-submitted after the current
	// agent turn completes (like Claude Code's /btw).
	steerQueue []string
}

// ToolAnimationState tracks the current animated tool display (yolo mode)
type ToolAnimationState struct {
	ToolName  string
	Command   string
	StartTime time.Time
	Frame     int
}

// ToolUseBlock represents an active tool invocation
type ToolUseBlock struct {
	ID   string
	Name string
}

// markdownRenderer is a lazy-initialized glamour renderer for markdown
func NewChatModel() ChatModel {
	ta := textarea.New()
	ta.SetHeight(MinInputRows)
	ta.SetWidth(80)
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.Placeholder = "Type a message..."
	ta.Focus()

	// CRITICAL FIX: Style the textarea to match our design system
	// This removes the strange background color inconsistency
	ta.Cursor.Style = lipgloss.NewStyle().Foreground(ColorPrimary)

	// Style the textarea base to have consistent background
	ta.FocusedStyle.Base = lipgloss.NewStyle().
		Background(ColorSurface).
		Foreground(ColorText)
	ta.BlurredStyle.Base = lipgloss.NewStyle().
		Background(ColorSurface).
		Foreground(ColorTextDim)

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

// SetCommandCompletions sets available slash commands for inline autocomplete.
func (m *ChatModel) SetCommandCompletions(commands []string) {
	m.allCommands = commands
}

// SetCommandDescriptions adds explanatory text to inline slash suggestions.
func (m *ChatModel) SetCommandDescriptions(descriptions map[string]string) {
	m.commandDescriptions = descriptions
}

// filterSuggestions returns commands matching the current input using fuzzy search.
// Results are ranked: exact prefix matches first, then substring matches, then
// fuzzy (edit distance) matches.
func (m *ChatModel) SetModel(model string) {
	m.model = model
}

// SetPersona sets the persona for contextual hints.
func (m *ChatModel) SetPersona(persona string) {
	m.persona = persona
}

// SetProvider sets the provider shown in the composer mode line.
func (m *ChatModel) SetProvider(provider string) {
	m.provider = provider
}

// SetEffort sets the reasoning effort shown in the composer mode line.
func (m *ChatModel) SetEffort(effort string) {
	m.effort = effort
}

// SetModeLabel sets the vim-mode label used when no persona is set.
func (m *ChatModel) SetModeLabel(label string) {
	m.modeLabel = label
}

// GetModel returns the model name.
func (m ChatModel) GetModel() string {
	return m.model
}

func (m *ChatModel) syncTextareaHeight() {
	rows := m.inputRows()
	if rows < MinInputRows {
		rows = MinInputRows
	}
	if rows > MaxInputRows {
		rows = MaxInputRows
	}
	m.textarea.SetHeight(rows)
}

func (m ChatModel) inputRows() int {
	value := m.textarea.Value()
	if value == "" {
		return MinInputRows
	}
	rows := strings.Count(value, "\n") + 1
	if rows > MaxInputRows {
		return MaxInputRows
	}
	return rows
}

func (m ChatModel) inputAreaHeight() int {
	// Border + top padding + editor rows + gap + mode line.
	height := 1 + ComposerTopPadding + m.inputRows() + ComposerGapRows + 1
	if m.thinking {
		height++ // thinking/streaming status line above the mode line
	}
	if m.showSuggestions && len(m.suggestions) > 0 {
		visible := len(m.suggestions)
		if visible > 6 {
			visible = 7 // six commands plus overflow hint
		}
		height += visible
	}
	return height
}

// Init initializes the chat model.
func (m *ChatModel) Focus() {
	m.focused = true
	m.textarea.Focus()
}

// Blur blurs the chat input.
func (m *ChatModel) Blur() {
	m.focused = false
	m.textarea.Blur()
	m.pendingSubmit = false
	m.pendingSubmitGen++
}

// AddMessage adds a message to the chat.
func (m *ChatModel) AddMessage(role, content string) {
	msg := ChatMessage{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}
	m.messages = append(m.messages, msg)
	m.refreshViewportFollow()
}

// SetMessages replaces the visible chat transcript from persisted session
// messages, preserving only user, assistant, system, and tool-result text.
func (m *ChatModel) SetMessages(messages []types.Message) {
	m.messages = make([]ChatMessage, 0, len(messages))
	for _, msg := range messages {
		chatMsg, ok := chatMessageFromSessionMessage(msg)
		if ok {
			m.messages = append(m.messages, chatMsg)
		}
	}
	m.refreshViewportFollow()
}

func chatMessageFromSessionMessage(msg types.Message) (ChatMessage, bool) {
	var content strings.Builder
	isTool := false
	toolName := ""
	status := ToolStatusComplete

	for _, block := range msg.Content {
		switch b := block.(type) {
		case types.TextBlock:
			if b.Text != "" {
				content.WriteString(b.Text)
			}
		case types.ToolUseBlock:
			isTool = true
			toolName = b.Name
			input := fmt.Sprintf("%v", b.Input)
			if input != "" && input != "map[]" {
				if content.Len() > 0 {
					content.WriteString("\n")
				}
				content.WriteString(fmt.Sprintf("→ %s %s", b.Name, input))
			} else {
				content.WriteString(fmt.Sprintf("→ %s", b.Name))
			}
		case types.ToolResultBlock:
			isTool = true
			if b.IsError {
				status = ToolStatusError
			}
			if content.Len() > 0 {
				content.WriteString("\n")
			}
			content.WriteString(fmt.Sprintf("%v", b.Content))
		}
	}

	text := strings.TrimSpace(content.String())
	if text == "" {
		return ChatMessage{}, false
	}

	role := string(msg.Role)
	if isTool {
		role = "tool"
	}
	return ChatMessage{
		ID:         msg.UUID,
		Role:       role,
		Content:    text,
		Timestamp:  msg.Timestamp,
		IsTool:     isTool,
		ToolName:   toolName,
		ToolStatus: status,
	}, true
}

// AddToolMessage adds a tool message to the chat.
// If you need message replacement (for live updates), use AddOrUpdateToolMessage instead.
func (m *ChatModel) SetInput(text string) {
	m.textarea.SetValue(text)
	m.syncTextareaHeight()
}

// GetInput returns the input text.
func (m ChatModel) GetInput() string {
	return m.textarea.Value()
}

// ClearInput clears the input.
func (m *ChatModel) ClearInput() {
	m.textarea.SetValue("")
	m.syncTextareaHeight()
}

// RemoveLastUserMessage removes the most recent user message from display.
func (m *ChatModel) RemoveLastUserMessage() {
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Role == "user" {
			m.messages = append(m.messages[:i], m.messages[i+1:]...)
			m.refreshViewportFollow()
			return
		}
	}
}

// QueueSteer adds a message to the steer queue. It will be auto-submitted as a
// user message after the current agent turn completes.
func (m *ChatModel) QueueSteer(text string) {
	m.steerQueue = append(m.steerQueue, text)
}

// GetSteerQueue returns the current steer queue (for testing).
func (m ChatModel) GetSteerQueue() []string {
	return m.steerQueue
}

// SetThinking sets the thinking state.
// When thinking is set to true, this also starts the response timer.
func (m ChatModel) ConsumesTab() bool {
	return m.showSuggestions
}

// ConsumesEsc returns whether this view consumes Esc key.
// When inline suggestions are showing, Esc dismisses them.
func (m ChatModel) ConsumesEsc() bool {
	return m.showSuggestions
}

// CapturesAllKeys returns whether this view should receive all keys
// before global shortcuts are applied.
func (m ChatModel) CapturesAllKeys() bool {
	return m.focused
}

// Scroll scrolls the viewport.
func (m *ChatModel) Scroll(lines int) {
	if lines > 0 {
		m.viewport.ScrollDown(lines)
	} else {
		m.viewport.ScrollUp(-lines)
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

// updateOrCreateStreamingMessage updates the assistant message for the current
// streaming turn or creates one. It uses currentStreamingAssistantIdx to track
// the exact message so that mid-stream system/user messages do not break the
// update target, while guaranteeing that a new user turn gets a fresh assistant
// message (fixing the history overwrite bug in Issue #4).
func (m *ChatModel) refreshViewport() {
	m.refreshViewportWithFollow(false)
}

func (m *ChatModel) refreshViewportFollow() {
	m.refreshViewportWithFollow(true)
}

func (m *ChatModel) refreshViewportWithFollow(forceBottom bool) {
	wasAtBottom := m.viewport.AtBottom()
	previousOffset := m.viewport.YOffset
	var content strings.Builder

	for _, msg := range m.messages {
		content.WriteString(m.renderMessage(msg))
		content.WriteString("\n\n")
	}

	// Show every completed tool from the current turn so users can see the
	// full execution chain, not just the most recent one.
	for i, toolMsg := range m.completedToolMsgs {
		if i > 0 || content.Len() > 0 {
			content.WriteString("\n\n")
		}
		content.WriteString(m.renderMessage(toolMsg))
	}

	// Add current tool message for in-place display (replaces previous running tool)
	if m.currentToolMsg != nil {
		if len(m.completedToolMsgs) > 0 || content.Len() > 0 {
			content.WriteString("\n\n")
		}
		content.WriteString(m.renderMessage(*m.currentToolMsg))
	}

	m.viewport.SetContent(content.String())
	if forceBottom || wasAtBottom {
		m.viewport.GotoBottom()
		return
	}
	m.viewport.SetYOffset(previousOffset)
}
