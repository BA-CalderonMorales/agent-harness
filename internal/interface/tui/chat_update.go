package tui

import (
	"fmt"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"os"
	"strings"
	"time"
)

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
		m.syncTextareaHeight()

		headerHeight := 2
		separatorHeight := 1
		inputHeight := m.inputAreaHeight()
		vpHeight := msg.Height - inputHeight - headerHeight - separatorHeight
		if vpHeight < 5 {
			vpHeight = 5
		}

		m.viewport.Width = msg.Width
		m.viewport.Height = vpHeight
		columnWidth := msg.Width
		if columnWidth > ComposerColumnWidth {
			columnWidth = ComposerColumnWidth
		}
		textareaWidth := columnWidth - 8
		if textareaWidth < 20 {
			textareaWidth = 20
		}
		m.textarea.SetWidth(textareaWidth)

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
					m.syncTextareaHeight()
					m.showSuggestions = false
					return m, nil
				}
			case "tab":
				if len(m.suggestions) > 0 {
					m.textarea.SetValue(m.suggestions[0] + " ")
					m.syncTextareaHeight()
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
			m.syncTextareaHeight()
			return m, nil
		}

		switch msg.Type {
		case tea.KeyEnter:
			if msg.Alt {
				// Multi-line input
				m.textarea.InsertString("\n")
				m.syncTextareaHeight()
				return m, nil
			}

			m.showSuggestions = false

			input := m.textarea.Value()
			if input == "" {
				return m, nil
			}

			// If a submit is already pending, this Enter is part of a paste stream.
			if m.pendingSubmit {
				m.pasteDetected = true
				m.textarea.InsertString("\n")
				m.syncTextareaHeight()
				return m, m.startSubmitTimer()
			}

			// Debounce: start submit timer. If another key arrives before the
			// timer fires, the Enter is treated as a pasted newline.
			if SubmitDebounceDuration <= 0 {
				return m.doSubmit()
			}
			m.pendingSubmit = true
			return m, m.startSubmitTimer()

		case tea.KeyCtrlC:
			if m.textarea.Value() != "" {
				m.textarea.SetValue("")
				m.pasteDetected = false
				m.pendingSubmit = false
				m.pendingSubmitGen++
				m.syncTextareaHeight()
			}
			return m, nil

		case tea.KeyCtrlJ:
			// Treat Ctrl+J (line feed) as newline insertion.
			// This preserves pasted newlines from terminals that send
			// raw LF instead of bracketed paste events.
			if m.pendingSubmit {
				m.pendingSubmit = false
				m.pendingSubmitGen++
			}
			m.textarea.InsertString("\n")
			m.syncTextareaHeight()
			return m, nil
		}

		// If another key arrives while a submit is pending, the previous
		// Enter was part of a paste stream — cancel the submit and insert
		// the newline that Enter would have represented.
		// Only do this for character keys (runes/space); control keys like
		// Backspace or Escape should simply cancel the pending submit.
		if m.pendingSubmit {
			m.pendingSubmit = false
			m.pendingSubmitGen++
			if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
				m.textarea.InsertString("\n")
				m.pasteDetected = true
				m.syncTextareaHeight()
			}
		}

		// Update textarea
		lastLen := len(m.textarea.Value())
		var newTA textarea.Model
		var cmd tea.Cmd
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Fprintf(os.Stderr, "[PANIC RECOVERED] textarea.Update: %v\n", r)
				}
			}()
			newTA, cmd = m.textarea.Update(msg)
		}()
		m.textarea = newTA
		m.syncTextareaHeight()

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
			return m, m.startTimer()
		}
		return m, nil

	// -------------------------------------------------------------------------
	// Submit debounce timer
	// -------------------------------------------------------------------------
	case submitTimerMsg:
		if !m.focused {
			return m, nil
		}
		if msg.generation == m.pendingSubmitGen && m.pendingSubmit {
			m.pendingSubmit = false
			return m.doSubmit()
		}
		return m, nil

	// -------------------------------------------------------------------------
	// Async agent messages - real-time streaming
	// -------------------------------------------------------------------------
	case AgentStartMsg:
		m.thinking = true
		m.thinkingText = "Thinking..."
		m.streaming = true
		m.turnInterrupted = false
		m.streamBuffer = ""
		m.currentStreamingAssistantIdx = -1
		m.startTime = time.Now()
		m.timerRunning = true
		m.elapsed = 0
		m.chunkCount = 0
		m.refreshViewport() // Ensure viewport scrolls to bottom after input height change
		return m, m.startTimer()

	case AgentConnectingMsg:
		// Show connecting state to user so they know we're trying
		m.thinking = true
		m.thinkingText = fmt.Sprintf("Connecting to %s...", msg.Endpoint)
		return m, nil

	case AgentChunkMsg:
		if m.streaming && !m.turnInterrupted {
			m.streamBuffer += msg.Text
			m.chunkCount++
			// Update or create the streaming assistant message
			m.updateOrCreateStreamingMessage(m.streamBuffer)
		}
		return m, nil

	case AgentToolStartMsg:
		if m.turnInterrupted {
			return m, nil
		}

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

		// MULTI-TOOL DISPLAY: Do NOT clear previous completed tools when a new tool
		// starts within the same turn. Users want to see the full chain of tool calls.

		toolMsg := ChatMessage{
			ID:              msg.ToolID,
			Role:            "tool",
			Content:         m.formatToolContent(displayName, command, ToolStatusRunning),
			Timestamp:       time.Now(),
			IsTool:          true,
			ToolName:        msg.ToolName,
			ToolDisplayName: displayName,
			ToolStatus:      ToolStatusRunning,
		}
		m.messages = append(m.messages, toolMsg)
		m.currentToolMsg = &m.messages[len(m.messages)-1]
		m.refreshViewport()
		return m, nil

	case AgentToolDoneMsg:
		if m.turnInterrupted {
			return m, nil
		}

		// Finalize the running tool message in place. Tool activity is part of the
		// transcript while work is happening, so the final assistant message stays last.
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
			m.completedToolMsgs = append(m.completedToolMsgs, *m.currentToolMsg)
			m.currentToolMsg = nil
		}
		m.currentTool = nil
		m.toolAnimation = nil
		m.refreshViewport()
		return m, nil

	case AgentDoneMsg:
		if m.turnInterrupted {
			return m, nil
		}

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

		m.completedToolMsgs = nil
		m.refreshViewport()

		// If there are queued steer messages, submit the next one automatically.
		if len(m.steerQueue) > 0 {
			steer := m.steerQueue[0]
			m.steerQueue = m.steerQueue[1:]
			m.AddMessage("user", steer)
			if m.delegate != nil {
				return m, m.delegate.OnSubmit(steer)
			}
		}
		return m, nil

	case AgentCancelMsg:
		if m.currentToolMsg != nil {
			command := m.extractCommandFromToolInput(m.currentToolMsg.ToolName, nil)
			if command == "" && m.toolAnimation != nil {
				command = m.toolAnimation.Command
			}
			m.currentToolMsg.Content = m.formatToolContent(m.currentToolMsg.ToolDisplayName, command, ToolStatusError)
			m.currentToolMsg.ToolStatus = ToolStatusError
			m.currentToolMsg.Timestamp = time.Now()
		}
		m.thinking = false
		m.streaming = false
		m.timerRunning = false
		m.turnInterrupted = true
		m.streamBuffer = ""
		m.currentStreamingAssistantIdx = -1
		m.currentTool = nil
		m.currentToolMsg = nil
		m.toolAnimation = nil
		m.completedToolMsgs = nil
		m.AddMessage("system", "Agent execution cancelled by user (ESC)")
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
		m.completedToolMsgs = nil
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
