// Chat view with rich message display and input handling

package tui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
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
)

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
	streaming    bool
	streamBuffer string
	currentTool  *ToolUseBlock

	// Timer state for response tracking
	startTime    time.Time
	elapsed      time.Duration
	timerRunning bool
	chunkCount   int

	// Tool animation state (for yolo mode - single animated line)
	toolAnimation *ToolAnimationState

	// Current tool message for in-place updates (replaces previous tool display)
	currentToolMsg *ChatMessage

	// completedToolMsg tracks the finalized tool message to display in history
	// This is separate from currentToolMsg to allow single-line replacement during execution
	completedToolMsg *ChatMessage

	// Delegate
	delegate ChatDelegate

	// Inline command suggestions (replaces modal palette)
	showSuggestions  bool
	suggestions      []string
	suggestionCursor int
	suggestionOffset int // scroll window start
	allCommands      []string

	// Paste detection state
	pasteDetected bool // true if current input was detected as a paste
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
var (
	markdownRenderer     *glamour.TermRenderer
	markdownRendererOnce sync.Once
	markdownRendererErr  error
	isTermux             = detectTermux()
)

// detectTermux checks if we're running in Termux environment
func detectTermux() bool {
	return os.Getenv("TERMUX_VERSION") != "" ||
		strings.Contains(os.Getenv("HOME"), "com.termux")
}

// getMarkdownRenderer returns a shared glamour renderer instance
func getMarkdownRenderer() (*glamour.TermRenderer, error) {
	markdownRendererOnce.Do(func() {
		markdownRenderer, markdownRendererErr = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(0), // We'll handle wrapping separately
		)
	})
	return markdownRenderer, markdownRendererErr
}

// renderMarkdown converts markdown text to ANSI-styled text
// In Termux, this returns plain text to avoid performance issues
func renderMarkdown(content string, width int) string {
	if strings.TrimSpace(content) == "" {
		return content
	}

	// Skip expensive markdown rendering in Termux for better performance
	if isTermux {
		return content
	}

	renderer, err := getMarkdownRenderer()
	if err != nil {
		// Fallback to plain text if renderer fails
		return content
	}

	// Render markdown to ANSI
	rendered, err := renderer.Render(content)
	if err != nil {
		return content
	}

	// Trim trailing newline that glamour adds
	rendered = strings.TrimSuffix(rendered, "\n")

	return rendered
}

// NewChatModel creates a new chat model.
// UI FIX: Styled textarea with consistent background for better visual appeal
func NewChatModel() ChatModel {
	ta := textarea.New()
	ta.SetHeight(3)
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

// filterSuggestions returns commands matching the current input.
func (m *ChatModel) filterSuggestions(input string) []string {
	if input == "" || input == "/" {
		return m.allCommands
	}
	var filtered []string
	query := strings.ToLower(input)
	for _, cmd := range m.allCommands {
		if strings.HasPrefix(strings.ToLower(cmd), query) {
			filtered = append(filtered, cmd)
		}
	}
	return filtered
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
		m.startTimer(), // Start the timer ticker
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

		// Detect bracketed paste from terminal
		if msg.Paste {
			m.pasteDetected = true
		}

		// Inline suggestion navigation
		if m.showSuggestions {
			switch msg.String() {
			case "down", "j":
				if m.suggestionCursor < len(m.suggestions)-1 {
					m.suggestionCursor++
					m.syncSuggestionOffset()
				}
				return m, nil
			case "up", "k":
				if m.suggestionCursor > 0 {
					m.suggestionCursor--
					m.syncSuggestionOffset()
				}
				return m, nil
			case "enter":
				if len(m.suggestions) > 0 && m.suggestionCursor < len(m.suggestions) {
					m.textarea.SetValue(m.suggestions[m.suggestionCursor] + " ")
					m.showSuggestions = false
					return m, nil
				}
			case "tab":
				if len(m.suggestions) > 0 {
					m.textarea.SetValue(m.suggestions[0] + " ")
					m.showSuggestions = false
					return m, nil
				}
			case "esc":
				m.showSuggestions = false
				return m, nil
			case "ctrl+c":
				m.showSuggestions = false
				return m, nil
			}
		}

		// Trigger inline suggestions when "/" is typed in empty input
		if msg.String() == "/" && m.textarea.Value() == "" {
			m.showSuggestions = true
			m.suggestions = m.filterSuggestions("/")
			m.suggestionCursor = 0
			m.textarea.InsertString("/")
			return m, nil
		}

		switch msg.Type {
		case tea.KeyEnter:
			if msg.Alt {
				// Multi-line input
				m.textarea.InsertString("\n")
				return m, nil
			}

			m.showSuggestions = false

			input := m.textarea.Value()
			if input == "" {
				return m, nil
			}

			// Handle slash commands
			trimmed := strings.TrimSpace(input)
			if strings.HasPrefix(trimmed, "/") {
				m.AddMessage("user", trimmed)
				if m.delegate != nil {
					m.delegate.OnCommand(trimmed)
				}
			} else {
				// Regular message: collapse pasted text in display when it is
				// multiline or exceeds the character threshold.
				displayText := input
				if m.pasteDetected {
					lineCount := strings.Count(input, "\n") + 1
					if lineCount > 1 {
						displayText = fmt.Sprintf("[Pasted text, %d lines, %d characters]", lineCount, len(input))
					} else if len(input) > PasteDisplayThreshold {
						displayText = fmt.Sprintf("[Pasted text, %d characters]", len(input))
					}
				}
				m.AddMessage("user", displayText)
				if m.delegate != nil {
					cmd := m.delegate.OnSubmit(input)
					m.textarea.SetValue("")
					m.pasteDetected = false
					m.refreshViewport()
					return m, cmd
				}
			}

			m.pasteDetected = false
			m.textarea.SetValue("")
			m.refreshViewport()
			return m, nil

		case tea.KeyCtrlC:
			// Allow Ctrl+C to propagate for quit
			m.pasteDetected = false
			return m, nil

		case tea.KeyCtrlJ:
			// Treat Ctrl+J (line feed) as newline insertion.
			// This preserves pasted newlines from terminals that send
			// raw LF instead of bracketed paste events.
			m.textarea.InsertString("\n")
			return m, nil
		}

		// Update textarea
		lastLen := len(m.textarea.Value())
		newTA, cmd := m.textarea.Update(msg)
		m.textarea = newTA

		// Heuristic paste detection for terminals without bracketed paste
		if !msg.Paste && len(m.textarea.Value())-lastLen > PasteHeuristicThreshold {
			m.pasteDetected = true
		}
		// Reset paste flag if input was cleared
		if len(m.textarea.Value()) == 0 {
			m.pasteDetected = false
		}

		cmds = append(cmds, cmd)

		// Refresh suggestions if showing
		if m.showSuggestions {
			val := m.textarea.Value()
			if !strings.HasPrefix(val, "/") || strings.Contains(val, " ") {
				m.showSuggestions = false
			} else {
				m.suggestions = m.filterSuggestions(val)
				m.suggestionCursor = 0
			}
		}

	// -------------------------------------------------------------------------
	// Timer tick for elapsed time display
	// -------------------------------------------------------------------------
	case timerTickMsg:
		if m.timerRunning {
			m.elapsed = time.Since(m.startTime)
		}
		// Always continue ticking - timer controls whether to update elapsed
		return m, m.startTimer()

	// -------------------------------------------------------------------------
	// Async agent messages - real-time streaming
	// -------------------------------------------------------------------------
	case AgentStartMsg:
		m.thinking = true
		m.thinkingText = "Thinking..."
		m.streaming = true
		m.streamBuffer = ""
		m.startTime = time.Now()
		m.timerRunning = true
		m.elapsed = 0
		m.chunkCount = 0
		return m, m.startTimer()

	case AgentConnectingMsg:
		// Show connecting state to user so they know we're trying
		m.thinking = true
		m.thinkingText = fmt.Sprintf("Connecting to %s...", msg.Endpoint)
		return m, nil

	case AgentChunkMsg:
		if m.streaming {
			m.streamBuffer += msg.Text
			m.chunkCount++
			// Update or create the streaming assistant message
			m.updateOrCreateStreamingMessage(m.streamBuffer)
		}
		return m, nil

	case AgentToolStartMsg:
		m.currentTool = &ToolUseBlock{ID: msg.ToolID, Name: msg.ToolName}
		displayName := msg.DisplayName
		if displayName == "" {
			displayName = msg.ToolName
		}

		// Use rich activity description from tool if available, otherwise extract from input
		command := msg.ActivityDesc
		if command == "" {
			command = m.extractCommandFromToolInput(msg.ToolName, msg.Input)
		}

		// Set up tool animation state for yolo-style display
		m.toolAnimation = &ToolAnimationState{
			ToolName:  displayName,
			Command:   command,
			StartTime: time.Now(),
			Frame:     0,
		}

		// SINGLE-LINE REPLACEMENT: Clear any previous completed tool before starting new one
		// This ensures only the current tool is visible (no scrolling from multiple tool lines)
		m.completedToolMsg = nil

		// Update current tool message for in-place display (replaces previous tool)
		m.currentToolMsg = &ChatMessage{
			ID:              msg.ToolID,
			Role:            "tool",
			Content:         m.formatToolContent(displayName, command, ToolStatusRunning),
			Timestamp:       time.Now(),
			IsTool:          true,
			ToolName:        msg.ToolName,
			ToolDisplayName: displayName,
			ToolStatus:      ToolStatusRunning,
		}
		m.refreshViewport()
		return m, nil

	case AgentToolDoneMsg:
		// Finalize current tool message and store as completed (but don't add to messages yet)
		// This enables single-line replacement: only the most recent completed tool is shown
		if m.currentToolMsg != nil && m.currentToolMsg.ID == msg.ToolID {
			status := ToolStatusSuccess
			if !msg.Success {
				status = ToolStatusError
			}
			command := m.extractCommandFromToolInput(m.currentToolMsg.ToolName, nil)
			if command == "" && m.toolAnimation != nil {
				command = m.toolAnimation.Command
			}
			m.currentToolMsg.Content = m.formatToolContent(m.currentToolMsg.ToolDisplayName, command, status)
			m.currentToolMsg.ToolStatus = status
			m.currentToolMsg.Timestamp = time.Now()

			// Store as completed tool message (replaces any previous completed tool)
			// This keeps the UI to a single line - only the most recent tool
			m.completedToolMsg = m.currentToolMsg
			m.currentToolMsg = nil
		}
		m.currentTool = nil
		m.toolAnimation = nil
		m.refreshViewport()
		return m, nil

	case AgentDoneMsg:
		m.thinking = false
		m.streaming = false
		// Finalize the streaming message
		// For streamed responses, use streamBuffer. For direct responses, use FullResponse
		finalContent := m.streamBuffer
		if finalContent == "" && msg.FullResponse != "" {
			finalContent = msg.FullResponse
		}
		if finalContent != "" {
			m.finalizeStreamingMessage(finalContent)
		}
		m.streamBuffer = ""

		// SINGLE-LINE REPLACEMENT: Commit the completed tool to message history
		// This preserves the final tool result in the conversation after the turn completes
		if m.completedToolMsg != nil {
			m.messages = append(m.messages, *m.completedToolMsg)
			m.completedToolMsg = nil
		}
		m.refreshViewport()
		return m, nil

	case AgentErrorMsg:
		m.thinking = false
		m.streaming = false

		// Build informative error message with action hints
		errStr := fmt.Sprintf("%v", msg.Error)
		var feedback string

		// Check for common error patterns and provide specific guidance
		switch {
		case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline"):
			feedback = "[!] Model timed out. The model may be overloaded or unresponsive.\n\n" +
				"[>] Try switching models: type /model <name> or press Tab to go to Settings\n" +
				"[?] Popular alternatives: claude-3-5-sonnet, gpt-4o, deepseek-chat"
		case strings.Contains(errStr, "connection") || strings.Contains(errStr, "network"):
			feedback = "[!] Connection error. Check your internet connection and API key.\n\n" +
				"[>] Verify settings: /config or Tab → Settings\n" +
				"[>] Check API key: /config"
		case strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "quota"):
			feedback = "[!] Rate limit or quota exceeded.\n\n" +
				"[>] Try a different model: /model <name>\n" +
				"[>] Check your account at your provider's dashboard"
		case strings.Contains(errStr, "authentication") || strings.Contains(errStr, "api key"):
			feedback = "[!] Authentication failed. Your API key may be invalid.\n\n" +
				"[>] Update API key: Tab → Settings → Provider\n" +
				"[>] Check /config for current settings"
		case strings.Contains(errStr, "model") && strings.Contains(errStr, "not found"):
			feedback = "[!] Model not found or unavailable.\n\n" +
				"[>] List available models: /model (with no args)\n" +
				"[>] Check supported models: /models or see docs/supported_models.md"
		default:
			// Generic error with helpful hints
			feedback = fmt.Sprintf("[!] Error: %s\n\n"+
				"[>] If the model isn't responding, try: /model <name>\n"+
				"[>] Or switch models via: Tab → Settings", errStr)
		}

		m.AddMessage("system", feedback)
		m.streamBuffer = ""
		return m, nil

	case ClearChatMsg:
		m.messages = make([]ChatMessage, 0)
		m.streamBuffer = ""
		m.thinking = false
		m.currentToolMsg = nil
		m.completedToolMsg = nil
		m.toolAnimation = nil
		m.currentTool = nil
		if msg.FollowUpMsg != "" {
			m.AddMessage("system", msg.FollowUpMsg)
		}
		m.refreshViewport()
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

	headerHeight := 2 // Header takes 2 lines
	separatorHeight := 1

	// Ensure minimum height for viewport
	vpHeight := m.height - inputHeight - headerHeight - separatorHeight
	if vpHeight < 5 {
		vpHeight = 5
	}

	// Ensure viewport has correct dimensions
	m.viewport.Width = m.width
	m.viewport.Height = vpHeight

	// Build the view
	var sections []string

	// Header (like Settings has)
	header := RenderHeader(HeaderConfig{
		Title:    "Chat",
		Subtitle: "Agent conversation",
		Count:    len(m.messages),
	})
	sections = append(sections, header)

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

	// Input area with styled container (golazo-inspired design)
	// CRITICAL FIX: Consistent styling for input bar
	inputContainer := InputContainerStyle.Width(m.width)

	var inputContent string
	prompt := PromptStyle.Render("◆ ")
	if m.thinking {
		statusLine := m.renderStatusLine()
		inputContent = prompt + m.textarea.View() + "\n" + statusLine
	} else {
		// Show model in status when not thinking
		modelDisplay := m.model
		if modelDisplay == "" {
			modelDisplay = "default"
		}
		statusLine := HelpDimStyle.Render(fmt.Sprintf("model: %s", modelDisplay))
		inputContent = prompt + m.textarea.View() + "\n" + statusLine
	}

	// Inline suggestions dropdown
	if m.showSuggestions && len(m.suggestions) > 0 {
		inputContent += "\n" + m.renderSuggestions()
	}

	sections = append(sections, inputContainer.Render(inputContent))

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// syncSuggestionOffset keeps cursor inside visible window.
func (m *ChatModel) syncSuggestionOffset() {
	maxVisible := 6
	if m.suggestionCursor < m.suggestionOffset {
		m.suggestionOffset = m.suggestionCursor
	}
	if m.suggestionCursor >= m.suggestionOffset+maxVisible {
		m.suggestionOffset = m.suggestionCursor - maxVisible + 1
	}
}

// renderSuggestions renders the inline suggestion dropdown.
func (m ChatModel) renderSuggestions() string {
	var b strings.Builder
	maxVisible := 6
	if len(m.suggestions) < maxVisible {
		maxVisible = len(m.suggestions)
	}

	start := m.suggestionOffset
	if start < 0 {
		start = 0
	}
	end := start + maxVisible
	if end > len(m.suggestions) {
		end = len(m.suggestions)
	}

	for i := start; i < end; i++ {
		sug := m.suggestions[i]
		indicator := "  "
		style := HelpDimStyle
		if i == m.suggestionCursor {
			indicator = IndicatorSelected + " "
			style = InfoStyle
		}
		b.WriteString(style.Render(indicator + sug))
		if i < end-1 {
			b.WriteString("\n")
		}
	}
	if len(m.suggestions) > maxVisible {
		b.WriteString("\n")
		if start > 0 {
			b.WriteString(HelpDimStyle.Render(fmt.Sprintf("  ...%d above", start)))
			if end < len(m.suggestions) {
				b.WriteString(HelpDimStyle.Render(fmt.Sprintf(" | %d below", len(m.suggestions)-end)))
			}
		} else {
			b.WriteString(HelpDimStyle.Render(fmt.Sprintf("  ...and %d more", len(m.suggestions)-end)))
		}
	}
	return b.String()
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
// If you need message replacement (for live updates), use AddOrUpdateToolMessage instead.
func (m *ChatModel) AddToolMessage(toolName, toolDisplayName, content string) {
	if toolDisplayName == "" {
		toolDisplayName = toolName
	}
	msg := ChatMessage{
		ID:              fmt.Sprintf("%s-%d", toolName, time.Now().UnixNano()),
		Role:            "tool",
		Content:         content,
		Timestamp:       time.Now(),
		IsTool:          true,
		ToolName:        toolName,
		ToolDisplayName: toolDisplayName,
		ToolStatus:      ToolStatusComplete,
	}
	m.messages = append(m.messages, msg)
	m.refreshViewport()
}

// AddOrUpdateToolMessage adds a tool message or updates existing one by ID.
// This prevents duplicate tool messages - instead updates in place.
func (m *ChatModel) AddOrUpdateToolMessage(id, toolName, toolDisplayName, command string, status ToolStatus) {
	if toolDisplayName == "" {
		toolDisplayName = toolName
	}

	// Build content with status indicator
	content := m.formatToolContent(toolDisplayName, command, status)

	// Look for existing message with same ID
	for i := range m.messages {
		if m.messages[i].ID == id && m.messages[i].IsTool {
			// Update existing message
			m.messages[i].Content = content
			m.messages[i].ToolStatus = status
			m.messages[i].Timestamp = time.Now()
			m.refreshViewport()
			return
		}
	}

	// Add new message
	msg := ChatMessage{
		ID:              id,
		Role:            "tool",
		Content:         content,
		Timestamp:       time.Now(),
		IsTool:          true,
		ToolName:        toolName,
		ToolDisplayName: toolDisplayName,
		ToolStatus:      status,
	}
	m.messages = append(m.messages, msg)
	m.refreshViewport()
}

// formatToolContent formats tool message content based on status
func (m *ChatModel) formatToolContent(toolDisplayName, command string, status ToolStatus) string {
	var statusIndicator string
	switch status {
	case ToolStatusRunning:
		statusIndicator = "→"
	case ToolStatusSuccess:
		statusIndicator = "✓"
	case ToolStatusError:
		statusIndicator = "✗"
	case ToolStatusComplete:
		statusIndicator = "✓"
	default:
		statusIndicator = "→"
	}

	if command != "" {
		return fmt.Sprintf("%s %s %s", statusIndicator, toolDisplayName, ToolCommandPreviewStyle.Render(command))
	}
	return fmt.Sprintf("%s %s", statusIndicator, toolDisplayName)
}

// AddToolMessageWithPreview adds a tool message with command preview.
// DEPRECATED: Use AddOrUpdateToolMessage instead for proper message replacement.
func (m *ChatModel) AddToolMessageWithPreview(toolName, toolDisplayName, command string) {
	m.AddOrUpdateToolMessage(toolName, toolName, toolDisplayName, command, ToolStatusRunning)
}

// extractCommandFromToolInput extracts the human-readable command from tool input
func (m *ChatModel) extractCommandFromToolInput(toolName string, input map[string]any) string {
	switch toolName {
	case "bash", "BashTool":
		if cmd, ok := input["command"].(string); ok && cmd != "" {
			// Truncate long commands for display
			return m.truncateCommand(cmd, 60)
		}
	case "read", "ReadTool":
		if path, ok := input["path"].(string); ok && path != "" {
			return fmt.Sprintf("cat %s", path)
		}
	case "write", "WriteTool":
		if path, ok := input["path"].(string); ok && path != "" {
			return fmt.Sprintf("write %s", path)
		}
	case "edit", "EditTool":
		if path, ok := input["path"].(string); ok && path != "" {
			return fmt.Sprintf("edit %s", path)
		}
	case "glob", "GlobTool":
		if pattern, ok := input["pattern"].(string); ok && pattern != "" {
			return fmt.Sprintf("find %s", pattern)
		}
	case "grep", "GrepTool":
		if pattern, ok := input["pattern"].(string); ok && pattern != "" {
			return fmt.Sprintf("grep '%s'", pattern)
		}
	case "webfetch", "WebFetchTool":
		if url, ok := input["url"].(string); ok && url != "" {
			return fmt.Sprintf("fetch %s", m.truncateCommand(url, 40))
		}
	case "websearch", "WebSearchTool":
		if query, ok := input["query"].(string); ok && query != "" {
			return fmt.Sprintf("search '%s'", m.truncateCommand(query, 40))
		}
	}
	return ""
}

// truncateCommand truncates a command for display with ellipsis
func (m *ChatModel) truncateCommand(cmd string, maxLen int) string {
	if len(cmd) <= maxLen {
		return cmd
	}
	return cmd[:maxLen-3] + "..."
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

// RemoveLastUserMessage removes the most recent user message from display.
func (m *ChatModel) RemoveLastUserMessage() {
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Role == "user" {
			m.messages = append(m.messages[:i], m.messages[i+1:]...)
			m.refreshViewport()
			return
		}
	}
}

// SetThinking sets the thinking state.
// When thinking is set to true, this also starts the response timer.
func (m *ChatModel) SetThinking(thinking bool, text string) {
	m.thinking = thinking
	m.thinkingText = text
	if text == "" {
		m.thinkingText = "Thinking..."
	}

	// Start/stop timer based on thinking state
	if thinking {
		m.startTime = time.Now()
		m.timerRunning = true
		m.elapsed = 0
		m.chunkCount = 0
	} else {
		m.timerRunning = false
	}
}

// startTimer returns a command that ticks every 100ms to update elapsed time
func (m *ChatModel) startTimer() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return timerTickMsg{time: t}
	})
}

// timerTickMsg is sent on each timer tick
type timerTickMsg struct {
	time time.Time
}

// renderStatusLine renders the thinking/streaming status with timer
func (m *ChatModel) renderStatusLine() string {
	if !m.thinking {
		return ""
	}

	// Calculate elapsed time
	if m.timerRunning {
		m.elapsed = time.Since(m.startTime)
	}

	// Format elapsed time
	elapsedStr := formatElapsed(m.elapsed)

	// If we have an active tool with animation state, show animated tool display
	// This provides the "yolo mode" single-line animated tool view
	if m.toolAnimation != nil && m.currentTool != nil {
		// Update animation frame
		m.toolAnimation.Frame++

		spinner := ToolSpinnerRender(m.toolAnimation.Frame)
		toolName := m.toolAnimation.ToolName
		command := m.toolAnimation.Command

		// Build animated tool line: spinner + tool name + grey command preview
		var statusParts []string
		statusParts = append(statusParts, InfoStyle.Render(spinner))
		statusParts = append(statusParts, ToolCallStyle.Render(toolName))
		if command != "" {
			statusParts = append(statusParts, ToolCommandPreviewStyle.Render(command))
		}
		statusParts = append(statusParts, HelpDimStyle.Render(fmt.Sprintf("(%s)", elapsedStr)))

		return strings.Join(statusParts, " ")
	}

	// Determine status text based on state
	status := "thinking"
	if m.streaming && m.streamBuffer != "" {
		status = "streaming"
	} else if m.currentTool != nil {
		status = fmt.Sprintf("using %s", m.currentTool.Name)
	}

	// Build status line: spinner + status + elapsed + chunks
	spinner := SpinnerRender("")
	detail := elapsedStr
	if m.chunkCount > 0 {
		detail = fmt.Sprintf("%s | %d chunks", elapsedStr, m.chunkCount)
	}

	statusText := StreamingStyle.Render(fmt.Sprintf("%s %s (%s)", strings.TrimSpace(spinner), status, detail))
	return statusText
}

// formatElapsed formats a duration as human-readable string
func formatElapsed(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	mins := int(d.Minutes())
	secs := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%ds", mins, secs)
}

// ConsumesTab returns whether this view consumes Tab key.
// When inline suggestions are showing, Tab is used for auto-completion.
func (m ChatModel) ConsumesTab() bool {
	return m.showSuggestions
}

// ConsumesEsc returns whether this view consumes Esc key.
// When inline suggestions are showing, Esc dismisses them.
func (m ChatModel) ConsumesEsc() bool {
	return m.showSuggestions
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
		// Update existing assistant message
		m.messages[len(m.messages)-1].Content = content
		m.messages[len(m.messages)-1].Timestamp = time.Now()
		m.messages[len(m.messages)-1].ResponseTime = m.elapsed
	} else {
		// Create new assistant message (for non-streamed responses)
		m.messages = append(m.messages, ChatMessage{
			Role:         "assistant",
			Content:      content,
			Timestamp:    time.Now(),
			ResponseTime: m.elapsed,
		})
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

	// SINGLE-LINE REPLACEMENT: Show only the most recent completed tool (if any)
	// This prevents multiple tool lines from accumulating and forcing scroll
	if m.completedToolMsg != nil {
		content.WriteString(m.renderMessage(*m.completedToolMsg))
	}

	// Add current tool message for in-place display (replaces previous running tool)
	if m.currentToolMsg != nil {
		if m.completedToolMsg != nil {
			content.WriteString("\n\n")
		}
		content.WriteString(m.renderMessage(*m.currentToolMsg))
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

	// Content - render markdown for rich formatting
	renderedContent := renderMarkdown(msg.Content, m.width-4)
	content := MessageBubbleUser.Width(m.width - 4).Render(renderedContent)
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
	// Show response time if available
	if msg.ResponseTime > 0 {
		header += SuccessStyle.Render(fmt.Sprintf(" (%s)", formatElapsed(msg.ResponseTime)))
	}
	b.WriteString(header)
	b.WriteString("\n")

	// Content - render markdown for rich formatting (code blocks, bold, italic, etc.)
	renderedContent := renderMarkdown(msg.Content, m.width-4)
	content := MessageBubbleAssistant.Width(m.width - 4).Render(renderedContent)
	b.WriteString(content)

	return b.String()
}

func (m ChatModel) renderToolMessage(msg ChatMessage) string {
	// Choose style based on tool status
	var style lipgloss.Style
	switch msg.ToolStatus {
	case ToolStatusRunning:
		style = ToolRunningStyle
	case ToolStatusSuccess, ToolStatusComplete:
		style = ToolDoneStyle
	case ToolStatusError:
		style = ToolErrorStyle
	default:
		style = ToolCallStyle
	}

	// Content already has status indicator and command preview from formatToolContent
	return style.Render(msg.Content)
}

func (m ChatModel) renderSystemMessage(msg ChatMessage) string {
	return SystemMessageStyle.Render(msg.Content)
}
