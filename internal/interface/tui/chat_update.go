package tui

import (
	"fmt"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
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
		m.resize(msg.Width, msg.Height)

	case tea.KeyMsg:
		// Key handling lives in chat_keys.go; it may mutate the model and
		// returns whether the key was fully consumed. Plain assignment:
		// ':=' would shadow the receiver and lose its mutations.
		var cmd tea.Cmd
		var handled bool
		m, cmd, handled = m.handleKeys(msg)
		if handled {
			return m, cmd
		}
		cmds = append(cmds, cmd)

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

		// Check for common error patterns and provide specific guidance.
		// Local providers never involve API keys, so their hints point at
		// the local model server instead.
		isLocal := m.provider == "local" || m.provider == "ollama"
		switch {
		case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline"):
			feedback = "[!] Model timed out. The model may be overloaded or unresponsive.\n\n" +
				"[>] Try switching models: type /model <name> or press Tab to go to Settings\n" +
				"[?] Popular alternatives: claude-3-5-sonnet, gpt-4o, deepseek-chat"
		case strings.Contains(errStr, "connection") || strings.Contains(errStr, "network"):
			if isLocal {
				feedback = "[!] Connection error. The local model server is not responding.\n\n" +
					"[>] Start it (e.g. llama-server or ollama) and verify the endpoint:\n" +
					"[>] Tab → Settings → Endpoint URL"
			} else {
				feedback = "[!] Connection error. Check your internet connection and API key.\n\n" +
					"[>] Verify settings: /config or Tab → Settings\n" +
					"[>] Check API key: /config"
			}
		case strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "quota"):
			feedback = "[!] Rate limit or quota exceeded.\n\n" +
				"[>] Try a different model: /model <name>\n" +
				"[>] Check your account at your provider's dashboard"
		case strings.Contains(errStr, "authentication") || strings.Contains(errStr, "api key"):
			if isLocal {
				feedback = "[!] Local provider does not use API keys.\n\n" +
					"[>] Verify the endpoint and model in: Tab → Settings"
			} else {
				feedback = "[!] Authentication failed. Your API key may be invalid.\n\n" +
					"[>] Update API key: Tab → Settings → Provider\n" +
					"[>] Check /config for current settings"
			}
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
