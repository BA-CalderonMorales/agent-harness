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
// viewportTopOffset counts the lines above the message viewport in the
// chat view: the app tab bar plus the chat view header.
const viewportTopOffset = 3

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
	// Mouse: click a tool line or a reasoning preview to expand its full
	// record; Esc or a second click folds it back. Wheel events scroll
	// the transcript.
	// -------------------------------------------------------------------------
	case tea.MouseMsg:
		if tea.MouseEvent(msg).IsWheel() {
			nv, cmd := m.viewport.Update(msg)
			m.viewport = nv
			return m, cmd
		}
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			// viewportTopOffset: app tab bar (1) + chat header (2) sits
			// above the message viewport.
			if id := m.expandableMessageAtRow(msg.Y - viewportTopOffset + m.viewport.YOffset); id != "" {
				if m.expandedMessageID == id {
					m.expandedMessageID = ""
				} else {
					m.expandedMessageID = id
				}
				m.refreshViewport()
			}
			return m, nil
		}

	// -------------------------------------------------------------------------
	// Timer tick for elapsed time display
	// -------------------------------------------------------------------------
	case timerTickMsg:
		if m.timerRunning {
			m.elapsed = time.Since(m.startTime)
			// After the placeholder delay, materialize the assistant
			// section (with whatever has buffered so far) so the thinking
			// header lags the question just a little.
			if m.placeholderPending && m.elapsed >= PlaceholderDelay {
				m.placeholderPending = false
				m.updateOrCreateStreamingMessage(m.streamBuffer)
			}
			// The thinking badge animates on this clock — without a
			// repaint per tick the ✦ twinkle and rotating quip freeze
			// until the next chunk happens to trigger a refresh.
			m.refreshViewport()
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
		m.thinkingIsStatus = true
		m.streaming = true
		m.turnInterrupted = false
		m.streamBuffer = ""
		m.currentStreamingAssistantIdx = -1
		m.startTime = time.Now()
		m.timerRunning = true
		m.elapsed = 0
		m.chunkCount = 0
		// New turn for tool-run grouping: collapsed runs are scoped to
		// one agent turn and never merge across turns.
		m.turnCounter++
		m.turnTools = nil
		// Defer the assistant section: it materializes after
		// PlaceholderDelay with whatever has buffered, so the thinking
		// header lags the question a little instead of popping instantly.
		m.placeholderPending = true
		m.refreshViewport() // Ensure viewport scrolls to bottom after input height change
		return m, m.startTimer()

	case AgentConnectingMsg:
		// Show connecting state to user so they know we're trying
		m.thinking = true
		m.thinkingText = fmt.Sprintf("Connecting to %s...", msg.Endpoint)
		m.thinkingIsStatus = true
		return m, nil

	case AgentThinkingMsg:
		// Reasoning preview from the agent goroutine: update the badge
		// text without touching the thinking timer.
		m.SetThinkingText(msg.Text)
		return m, nil

	case AgentSystemNoteMsg:
		m.AddMessage("system", msg.Text)
		return m, nil

	case AgentChunkMsg:
		if m.streaming && !m.turnInterrupted {
			m.streamBuffer += msg.Text
			m.chunkCount++
			// During the placeholder delay, chunks buffer quietly; the
			// assistant section materializes on the next tick.
			if !m.placeholderPending {
				// Update or create the streaming assistant message
				m.updateOrCreateStreamingMessage(m.streamBuffer)
			}
		}
		return m, nil

	case AgentToolStartMsg:
		if m.turnInterrupted {
			return m, nil
		}

		// The prose before this call is where the call actually
		// happened: mark the buffer offset — parts derive from it.
		m.turnTools = append(m.turnTools, turnToolMark{ToolID: msg.ToolID, At: len(m.streamBuffer)})

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
			Content:         m.formatToolContent(displayName, command, ToolStatusRunning, time.Now(), 0),
			Timestamp:       time.Now(),
			IsTool:          true,
			ToolName:        msg.ToolName,
			ToolDisplayName: displayName,
			ToolStatus:      ToolStatusRunning,
			ToolStartedAt:   time.Now(),
			ToolDetail:      command,
			Turn:            m.turnCounter,
		}
		m.appendToolMessage(toolMsg)
		// Re-find by ID: the insertion shifts slice positions, so a
		// cached pointer could alias the wrong slot.
		for i := range m.messages {
			if m.messages[i].ID == msg.ToolID && m.messages[i].IsTool {
				m.currentToolMsg = &m.messages[i]
				break
			}
		}
		m.refreshViewport()
		return m, nil

	case AgentToolDoneMsg:
		if m.turnInterrupted {
			return m, nil
		}

		// Finalize the tool message in the transcript by ID: tools from
		// earlier in the turn are still in m.messages, and a stale
		// "Running" status would otherwise stick forever (the collapse
		// only folds final-status tools, so the transcript must carry
		// the truth). Tool activity is part of the transcript while work
		// is happening, so the final assistant message stays last.
		status := ToolStatusSuccess
		if !msg.Success {
			status = ToolStatusError
		}
		for i := range m.messages {
			if m.messages[i].ID == msg.ToolID && m.messages[i].IsTool {
				command := m.extractCommandFromToolInput(m.messages[i].ToolName, nil)
				if command == "" && m.toolAnimation != nil && m.currentToolMsg != nil &&
					m.currentToolMsg.ID == msg.ToolID {
					command = m.toolAnimation.Command
				}
				if m.messages[i].ToolStartedAt.IsZero() {
					m.messages[i].ToolStartedAt = m.messages[i].Timestamp
				}
				detail := command
				if detail == "" {
					detail = m.messages[i].ToolDetail
				} else {
					m.messages[i].ToolDetail = detail
				}
				m.messages[i].ToolElapsed = time.Since(m.messages[i].ToolStartedAt)
				m.messages[i].Content = m.formatToolContent(m.messages[i].ToolDisplayName, detail, status, m.messages[i].ToolStartedAt, m.messages[i].ToolElapsed)
				m.messages[i].ToolStatus = status
				break
			}
		}
		if m.currentToolMsg != nil && m.currentToolMsg.ID == msg.ToolID {
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
		m.placeholderPending = false
		// Finalize the streaming message
		// For streamed responses, use streamBuffer. For direct responses, use FullResponse
		finalContent := m.streamBuffer
		if finalContent == "" && msg.FullResponse != "" {
			finalContent = msg.FullResponse
		}
		// Tool-only turns stream no text: the placeholder already holds the
		// tool display, so settle it in place instead of leaving a stuck
		// assistant bubble behind.
		if finalContent == "" {
			if msg := m.streamingAssistant(); msg != nil {
				finalContent = msg.Content
			}
		}
		if finalContent != "" {
			m.finalizeStreamingMessage(finalContent)
		}
		m.streamBuffer = ""
		m.currentToolMsg = nil
		m.currentTool = nil
		m.toolAnimation = nil

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
			started := m.currentToolMsg.ToolStartedAt
			if started.IsZero() {
				started = m.currentToolMsg.Timestamp
			}
			m.currentToolMsg.ToolElapsed = time.Since(started)
			m.currentToolMsg.Content = m.formatToolContent(m.currentToolMsg.ToolDisplayName, command, ToolStatusError, started, m.currentToolMsg.ToolElapsed)
			m.currentToolMsg.ToolStatus = ToolStatusError
		}
		m.thinking = false
		m.streaming = false
		m.timerRunning = false
		m.turnInterrupted = true
		m.streamBuffer = ""
		m.placeholderPending = false
		m.dropPlaceholderIfEmpty()
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
		m.placeholderPending = false
		// Finalize the in-progress message so the thinking spinner does not
		// stay stuck on its header; drop it entirely if nothing arrived.
		if strings.TrimSpace(m.streamBuffer) == "" {
			m.dropPlaceholderIfEmpty()
		} else {
			m.finalizeStreamingMessage(m.streamBuffer)
		}

		// Build informative error message with action hints.
		errStr := fmt.Sprintf("%v", msg.Error)
		isLocal := m.provider == "local" || m.provider == "ollama"
		feedback := ProviderErrorFeedback(ClassifyProviderError(errStr), errStr, isLocal)
		m.AddMessage("system", feedback)
		m.streamBuffer = ""
		return m, nil

	case ClearChatMsg:
		m.messages = make([]ChatMessage, 0)
		m.streamBuffer = ""
		m.thinking = false
		m.placeholderPending = false
		m.currentToolMsg = nil
		m.completedToolMsgs = nil
		m.toolAnimation = nil
		m.currentTool = nil
		// A wiped pane is a fresh first run: the clear's follow-up
		// notice lands first, then the navigation guidance rides with it.
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
