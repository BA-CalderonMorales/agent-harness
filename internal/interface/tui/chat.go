// Chat view with rich message display and input handling

package tui

import (
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
// TurnPart is one slice of an assistant response: a prose run or the
// tool calls that interrupted it.
type TurnPart struct {
	Text   string // non-empty for a prose part
	ToolID string // non-empty for a tool part; matches the tool ChatMessage ID
}

// turnToolMark records where a tool call interrupted the stream: the
// call's message ID and its offset into the turn's stream buffer. The
// prose parts derive from those offsets — the buffer stays whole, so
// saving and finalizing keep their semantics.
type turnToolMark struct {
	ToolID string
	At     int
}

type ChatMessage struct {
	ID              string // Unique identifier for message replacement
	Role            string
	Content         string
	Timestamp       time.Time
	IsTool          bool
	ToolName        string
	ToolDisplayName string        // User-friendly display name for the tool
	ToolStatus      ToolStatus    // pending, running, success, error
	ToolStartedAt   time.Time     // When the tool call began (drives the log timestamp)
	ToolElapsed     time.Duration // Run-to-settle duration, filled on the terminal status
	ToolDetail      string        // Target of the call (command, path, pattern) for the detail column
	ToolInputJSON   string        // Full tool input as JSON, for the expandable detail view
	ReasoningText   string        // Model reasoning for this message (in-memory only, never persisted)
	ResponseTime    time.Duration // Time taken to generate this response
	StreamedChunks  int           // Token chunks streamed for this response
	Thinking        bool          // In-progress response (drives the live spinner header)
	Turn            int           // Agent turn that produced this message (tool-run grouping key)

	// Parts segments the response where tool calls interrupted it:
	// prose runs alternate with the tool calls that followed them.
	// Empty on legacy data — Content renders whole. Tool parts resolve
	// to the tool ChatMessage with the matching ID.
	Parts []TurnPart
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

// clickRange records which viewport rows of a rendered block resolve a
// mouse click back to the message that rendered there. Tool blocks map
// their whole block; assistant blocks map only their reasoning rows.
type clickRange struct {
	start, end int // inclusive viewport content rows
	msgID      string
}

// Paste detection thresholds.
const (
	PasteDisplayThreshold   = 200 // min chars to collapse a pasted message
	PasteHeuristicThreshold = 20  // min length jump in one keystroke to detect paste
	MinInputRows            = 1
	MaxInputRows            = 4
)

// Composer layout: the input block spans the full terminal width with a
// little breathing room above the text. The solid surface block hugs the
// text (the editor grows with the lines); the mode line (mode · model ·
// provider · reasoning effort) renders below the block on the terminal
// background.
const (
	ComposerTopPadding    = 1 // blank rows above the input text
	ComposerBottomPadding = 1 // blank rows below the input text, inside the block
)

// PlaceholderDelay is how long the agent section waits before appearing
// after a question is submitted. Chunks arriving during the delay are
// buffered quietly; the header then pops in with the spinning ◆ thinking
// indicator instead of an instant, jarring response row.
const PlaceholderDelay = 1 * time.Second

// SubmitDebounceDuration is the window after Enter during which another
// keystroke may still influence how the Enter is interpreted.
var SubmitDebounceDuration = 80 * time.Millisecond

// PasteBurstThreshold separates machine-speed paste bursts from a fast
// typist. Keys inside a terminal paste stream arrive microseconds apart;
// even a hammering typist needs tens of milliseconds per key. A keystroke
// landing within the threshold of the pending Enter continues a paste
// (newline insertion); anything later is a human — Enter's contract
// (submit) wins and the keystroke starts the next message.
const PasteBurstThreshold = 20 * time.Millisecond

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
	// thinkingIsStatus marks thinkingText as loop status ("Thinking...",
	// "Connecting to …") rather than model reasoning: status must drive
	// the badge, never the reasoning record — the expanded reasoning
	// frame once rendered "Connecting to local..." as the model's
	// thoughts.
	thinkingIsStatus bool
	model            string

	// Composer mode line metadata (set by the app-level runtime context)
	provider  string
	effort    string
	modeLabel string // "typing" / "navigate" when persona is empty
	agentMode string // composer agent mode chip; "" hides it
	// Streaming state
	streaming       bool
	streamBuffer    string
	currentTool     *ToolUseBlock
	turnInterrupted bool

	// Index of the assistant message currently being streamed. Tracked so that
	// mid-stream system/user messages do not break the update target, while
	// ensuring a new user turn always gets a fresh assistant message. The
	// index DRIFTS when a mid-turn PrependSystemNote (provider probe,
	// auto-save notice) inserts at position 0 - message lookup goes by
	// currentStreamingAssistantID, and the index is maintained alongside
	// for readers.
	currentStreamingAssistantIdx int
	currentStreamingAssistantID  string

	// Timer state for response tracking
	startTime    time.Time
	elapsed      time.Duration
	timerRunning bool
	chunkCount   int

	// Tool animation state (for yolo mode - single animated line)
	toolAnimation *ToolAnimationState

	// Current tool message for in-place updates (replaces previous tool display)
	currentToolMsg *ChatMessage

	// lastComposerTop is the composer's top row in pane coordinates,
	// recorded by View for the tap-to-type click mapping.
	lastComposerTop int

	// completedToolMsgs tracks all finalized tool messages for the current turn.
	// Previously this was a single pointer for single-line replacement, but users
	// want to see every tool call that happens during a conversation turn. The
	// render path reads the transcript (m.messages) instead of these copies -
	// they exist for state inspection and are cleared per turn.
	completedToolMsgs []ChatMessage

	// turnTools marks where tool calls interrupted the streaming
	// response; the streaming assistant message carries the parts.
	turnTools []turnToolMark

	// turnCounter stamps tool messages with their agent turn so collapsed
	// tool runs never merge across turn boundaries.
	turnCounter int

	// toolsCollapsed renders consecutive same-tool runs as one count line
	// (t toggles); errors, approvals, and running tools never collapse.
	toolsCollapsed bool

	// first Chat entry of a session and again after /clear only.

	// clickIndex maps viewport rows to the messages a click resolves
	// to — tool blocks and reasoning preview/frame rows — rebuilt on
	// every refresh.
	clickIndex []clickRange

	// lastPainted / lastPaintedAtBottom dedupe refreshes: the tick-
	// driven streaming repaint skips SetContent when the built
	// transcript is unchanged.
	lastPainted         string
	lastPaintedAtBottom bool

	// expandedMessageID is the message whose full record is expanded
	// inline (click the line, or Enter on the latest; Esc closes). It
	// covers tool calls and the model's reasoning alike.
	expandedMessageID string

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
	pasteDetected bool              // true if current input was detected as a paste
	pendingPastes map[string]string // collapsed paste tokens -> full content

	// Debounce state for distinguishing intentional Enter from pasted newlines
	pendingSubmit    bool
	pendingSubmitGen int
	pendingAt        time.Time // when the pending Enter landed; burst discriminator

	// placeholderPending defers the assistant message (and its thinking
	// header) until PlaceholderDelay has elapsed since the question.
	placeholderPending bool

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

// cursorStyle is the composer caret: terminal surface under a primary
// block. It re-derives from the live palette so a theme switch moves
// the cursor with it — the style was captured once at construction and
// used to keep the boot theme's colors forever.
func cursorStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ColorSurface).Background(ColorPrimary)
}

// refreshCursorStyle re-applies the caret colors after a theme switch.
func (m *ChatModel) refreshCursorStyle() {
	m.textarea.Cursor.Style = cursorStyle()
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

	// Style the textarea to match our design system. The base carries the
	// Terminal-native composer: the textarea paints no background of
	// its own — the top rule above and the mode line below bound the
	// typing area, and every cell stays on the terminal's surface.
	// bubbles' defaults paint the cursor row black (a partial stripe
	// behind typed text); clear every row-level background.
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.Cursor.Style = cursorStyle()
	ta.FocusedStyle.Base = lipgloss.NewStyle().Foreground(ColorText)
	ta.BlurredStyle.Base = lipgloss.NewStyle().Foreground(ColorTextDim)

	vp := newViewport(80, 20)

	return ChatModel{
		textarea: ta,
		viewport: vp,
		messages: make([]ChatMessage, 0),
		// The chat model is typing-ready by construction; the App blurs
		// the composer at boot so navigate mode owns the keyboard.
		focused: true,
		// Tool runs render collapsed by default: the wall of identical
		// tool lines is the long-horizon reading pain, not the collapse.
		toolsCollapsed: true,
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

// SetAgentMode sets the agent mode chip shown in the composer mode line.
// Empty hides the chip (boot before the mode is synced).
func (m *ChatModel) SetAgentMode(mode string) {
	m.agentMode = mode
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
	// The solid block: border + top padding + editor rows + bottom padding.
	// The mode line below the block adds one more row to the reserved area.
	height := 1 + ComposerTopPadding + m.inputRows() + ComposerBottomPadding + 1
	if m.showSuggestions && len(m.suggestions) > 0 {
		visible := len(m.suggestions)
		if visible > 6 {
			visible = 7 // six commands plus overflow hint
		}
		height += visible
	}
	return height
}

// Focus focuses the chat input and returns it to the typing affordance.
func (m *ChatModel) Focus() {
	m.focused = true
	m.textarea.Focus()
	m.togglePlaceholder()
}

// Blur blurs the chat input and turns the composer into a vim-style
// navigate affordance: the placeholder teaches the 'i' key.
func (m *ChatModel) Blur() {
	m.focused = false
	m.textarea.Blur()
	m.pendingSubmit = false
	m.pendingSubmitGen++
	m.togglePlaceholder()
}

// togglePlaceholder keeps the composer honest about its mode: in navigate
// mode the placeholder says what to press; in typing mode it invites text.
func (m *ChatModel) togglePlaceholder() {
	if m.focused {
		m.textarea.Placeholder = "Type a message..."
	} else {
		m.textarea.Placeholder = `"i" to type a message`
	}
}
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
