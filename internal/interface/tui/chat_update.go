package tui

import (
	"encoding/json"
	"fmt"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	"time"
)

func (m ChatModel) Init() tea.Cmd {
	return textarea.Blink
}

// settleCurrentTool records a terminal outcome when the provider stops
// without sending the normal ToolDone event (for example, a transport
// error). It deliberately keeps currentToolMsg intact for the caller's
// cleanup path.
func (m *ChatModel) settleCurrentTool(status ToolStatus) {
	toolID := ""
	if m.currentToolMsg != nil {
		toolID = m.currentToolMsg.ID
	}
	if toolID == "" && m.currentTool != nil {
		toolID = m.currentTool.ID
	}
	if toolID == "" {
		return
	}
	for i := range m.messages {
		msg := &m.messages[i]
		if !msg.IsTool || msg.ID != toolID || msg.ToolStatus == ToolStatusSuccess || msg.ToolStatus == ToolStatusError || msg.ToolStatus == ToolStatusComplete {
			continue
		}
		started := msg.ToolStartedAt
		if started.IsZero() {
			started = msg.Timestamp
		}
		msg.ToolElapsed = time.Since(started)
		command := msg.ToolDetail
		if command == "" && m.toolAnimation != nil {
			command = m.toolAnimation.Command
		}
		msg.ToolDetail = command
		msg.ToolStatus = status
		msg.Content = m.formatToolContent(msg.ToolDisplayName, command, shortToolTag(msg.ID), status, started, msg.ToolElapsed)
		msg.bumpRev()
		return
	}
}

// Update handles messages.
// viewportTopOffset counts the screen rows above the message viewport in
// the chat view: the app frame's top border (1), the tab bar (padding
// row, label row, border row), and the chat view header (title row,
// blank row).
const viewportTopOffset = 1 + 5

func (m ChatModel) Update(msg tea.Msg) (model tea.Model, cmd tea.Cmd) {
	// Deferred rebuilds flush on every exit path: the returned model
	// is what BubbleTea persists, so the rebuilt viewport content,
	// clickIndex, and the cleared refreshPending flag must survive the
	// frame. (A flush in View would mutate only a value-receiver copy
	// — stale clickIndex and a forever-pending flag.) Handlers may
	// defer mid-handling and early-return; the defer catches all of
	// them.
	defer func() {
		if cm, ok := model.(ChatModel); ok {
			cm.flushDeferredRefresh()
			model = cm
		}
	}()

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
			// Flush any pending rebuild first: scrolling stale content
			// silently loses the wheel event's effect when the rebuild
			// lands on the next frame.
			m.flushDeferredRefresh()
			if isMobilePane(m.width) {
				// Phone rows are taller, so a fixed 3-line tick
				// crawls: scroll by a viewport fraction for a
				// snappy feel that matches the device.
				step := m.viewport.Height / 3
				if step < 1 {
					step = 1
				}
				switch tea.MouseEvent(msg).Button {
				case tea.MouseButtonWheelDown:
					m.viewport.ScrollDown(step)
				case tea.MouseButtonWheelUp:
					m.viewport.ScrollUp(step)
				}
				return m, nil
			}
			m.viewport.Update(msg)
			return m, nil
		}
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			// viewportTopOffset: the tab bar (3 rows) plus the chat header
			// (2 rows) sit above the message viewport.
			if id := m.expandableMessageAtRow(msg.Y - viewportTopOffset + m.viewport.YOffset); id != "" {
				if m.expandedMessageID == id {
					m.expandedMessageID = ""
				} else {
					m.expandedMessageID = id
				}
				m.refreshViewport()
				return m, nil
			}
			// Tap-to-type: a press on the composer asks to type. The mode
			// is App state — the request rides a message so the mode line
			// stays the truth.
			if m.lastComposerTop > 0 && msg.Y >= m.lastComposerTop && !m.focused {
				return m, func() tea.Msg { return ComposerFocusMsg{} }
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
			// until the next chunk happens to trigger a refresh. The
			// tick is also what flushes chunk-deferred rebuilds: one
			// assembly per tick, not per chunk.
			m.refreshDeferred()
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
		// Stamp the note with the in-flight turn so the turn block can
		// carry it as an inline row instead of splitting the tool burst
		// (goal 0.3.29 Task 3a — the [Tool loop detected: ...] case).
		if m.streaming {
			if n := len(m.messages); n > 0 && m.messages[n-1].Role == "system" {
				m.messages[n-1].Turn = m.turnCounter
				m.messages[n-1].bumpRev()
			}
		}
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

		// Materialize the streaming assistant before the tool lands:
		// a tool that starts inside the placeholder window (fast
		// models, instant commands) would otherwise orphan its row
		// below a bubble that doesn't exist yet — the user sees the
		// working line spin with nothing in the transcript (goal
		// 0.3.29 live-visibility finding). The acknowledgment lands
		// with the first tool call, which is exactly when the user
		// needs to see what is happening.
		if m.placeholderPending {
			m.placeholderPending = false
			m.updateOrCreateStreamingMessage(m.streamBuffer)
		}
		// Providers may repeat a start notification while retrying or
		// forwarding events. The transcript is the authority: keep the
		// original row and current part, rather than creating a second
		// visible call for the same tool ID.
		for i := range m.messages {
			if m.messages[i].IsTool && m.messages[i].ID == msg.ToolID {
				m.currentTool = &ToolUseBlock{ID: msg.ToolID, Name: msg.ToolName}
				m.currentToolMsg = &m.messages[i]
				m.refreshViewport()
				return m, nil
			}
		}

		// The prose before this call is where the call actually
		// happened: mark the buffer offset — parts derive from it.
		m.turnTools = append(m.turnTools, turnToolMark{ToolID: msg.ToolID, At: len(m.streamBuffer)})

		// Re-derive the streaming message's parts NOW: the tool row
		// must render inside the live turn bubble from the first
		// call, not on the next chunk (an LLM gone quiet while a
		// command runs would leave the call outside the bubble for
		// the whole execution).
		if smsg := m.streamingAssistant(); smsg != nil {
			smsg.Parts = m.deriveParts(m.streamBuffer)
			smsg.bumpRev()
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

		// Carry the raw input JSON: the expandable record and the todo
		// checklist renderer read from it (never populated before
		// 0.3.28, so expanded records showed no input).
		inputJSON := ""
		if len(msg.Input) > 0 {
			if raw, err := json.Marshal(msg.Input); err == nil {
				inputJSON = string(raw)
			}
		}

		toolMsg := ChatMessage{
			ID:              msg.ToolID,
			Role:            "tool",
			Content:         m.formatToolContent(displayName, command, shortToolTag(msg.ToolID), ToolStatusRunning, time.Now(), 0),
			Timestamp:       time.Now(),
			IsTool:          true,
			ToolName:        msg.ToolName,
			ToolDisplayName: displayName,
			ToolStatus:      ToolStatusRunning,
			ToolStartedAt:   time.Now(),
			ToolDetail:      command,
			ToolInputJSON:   inputJSON,
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
				m.messages[i].Content = m.formatToolContent(m.messages[i].ToolDisplayName, detail, shortToolTag(m.messages[i].ID), status, m.messages[i].ToolStartedAt, m.messages[i].ToolElapsed)
				m.messages[i].bumpRev()
				m.messages[i].ToolStatus = status
				break
			}
		}
		if m.currentToolMsg != nil && m.currentToolMsg.ID == msg.ToolID {
			m.completedToolMsgs = append(m.completedToolMsgs, *m.currentToolMsg)
			m.currentToolMsg = nil
		}
		// Re-derive the live bubble's parts: the call just settled, so
		// its row must flip to the settled state inside the bubble (the
		// group-cache signature reads len(Parts) — without this the
		// cached block keeps the running row).
		if smsg := m.streamingAssistant(); smsg != nil {
			smsg.Parts = m.deriveParts(m.streamBuffer)
			smsg.bumpRev()
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
		m.timerRunning = false
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
			m.currentToolMsg.Content = m.formatToolContent(m.currentToolMsg.ToolDisplayName, command, shortToolTag(m.currentToolMsg.ID), ToolStatusError, started, m.currentToolMsg.ToolElapsed)
			m.currentToolMsg.bumpRev()
			m.currentToolMsg.ToolStatus = ToolStatusError
		}
		m.thinking = false
		m.streaming = false
		m.timerRunning = false
		m.turnInterrupted = true
		m.streamBuffer = ""
		m.placeholderPending = false
		// A streaming assistant message with content must finalize with
		// Thinking=false: left as-is, it renders the animated thinking
		// badge with a frozen elapsed time forever — and the next turn's
		// first events inherit the dead turn's live-header state in the
		// group cache. Empty placeholders are dropped outright.
		if msg := m.streamingAssistant(); msg != nil {
			if strings.TrimSpace(msg.Content) == "" {
				m.dropPlaceholderIfEmpty()
			} else {
				msg.Thinking = false
				msg.ResponseTime = m.elapsed
				msg.bumpRev()
				m.currentStreamingAssistantID = ""
				m.currentStreamingAssistantIdx = -1
				m.refreshViewport()
			}
		} else {
			m.dropPlaceholderIfEmpty()
		}
		m.currentTool = nil
		m.currentToolMsg = nil
		m.toolAnimation = nil
		m.completedToolMsgs = nil
		m.AddMessage("system", "Agent execution cancelled by user (ESC)")
		m.refreshViewport()
		return m, nil

	case AgentErrorMsg:
		// A provider error can arrive without a ToolDone event. Settle
		// the active row first so the transcript never claims that a
		// command is still running after the turn has stopped.
		m.settleCurrentTool(ToolStatusError)
		// Provider errors terminate this turn. Ignore any already queued
		// tool/result events so a late completion cannot overwrite the
		// truthful error state.
		m.turnInterrupted = true
		m.thinking = false
		m.streaming = false
		m.timerRunning = false
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
		m.streaming = false
		m.timerRunning = false
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
	m.viewport.Update(msg)

	return m, tea.Batch(cmds...)
}
