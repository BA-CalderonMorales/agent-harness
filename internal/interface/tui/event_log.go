package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// eventLog is an opt-in observation stream for terminal journey drivers.
// It never decides permissions or changes the messages the application handles.
// Records contain user/model/tool text; callers must choose a private location.
type eventLog struct {
	file *os.File
	err  error
	turn int
}

func openEventLog(path string) (*eventLog, error) {
	if path == "" {
		return nil, nil
	}
	// Refuse existing paths, including symlinks. A new run owns its log.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("create terminal event log: %w", err)
	}
	return &eventLog{file: f}, nil
}

func (l *eventLog) filter(_ tea.Model, msg tea.Msg) tea.Msg {
	r := map[string]any{"schema_version": 1, "at": time.Now().UTC(), "turn": l.turn}
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		r["type"], r["width"], r["height"] = "window", m.Width, m.Height
	case UserSubmitMsg:
		r["type"], r["text"] = "submit", m.Text
	case AgentStartMsg:
		l.turn++
		r["type"], r["turn"] = "start", l.turn
	case AgentChunkMsg:
		r["type"], r["text"] = "text", m.Text
	case AgentDoneMsg:
		r["type"], r["text"], r["tool_calls"] = "complete", m.FullResponse, m.ToolCalls
	case AgentErrorMsg:
		r["type"], r["error"] = "error", fmt.Sprint(m.Error)
	case AgentToolStartMsg:
		r["type"], r["id"], r["tool"], r["input"] = "tool_start", m.ToolID, m.ToolName, m.Input
	case AgentToolDoneMsg:
		r["type"], r["id"], r["success"], r["output"] = "tool_done", m.ToolID, m.Success, m.Output
	case ApprovalRequestMsg:
		if m.Request == nil {
			return msg
		}
		r["type"], r["id"], r["tool"], r["command"] = "approval", m.Request.Command.ID, m.Request.Command.ToolName, m.Request.Command.Command
		r["input"] = m.Input
	case AgentCancelMsg:
		r["type"] = "cancel"
	case QuestionRequestMsg:
		r["type"] = "question"
	default:
		return msg
	}
	if err := json.NewEncoder(l.file).Encode(r); err != nil {
		l.err = fmt.Errorf("write terminal event log: %w", err)
		return tea.QuitMsg{}
	}
	return msg
}

// openEventPipe streams observations to a host-owned receiver. The application
// never opens the authoritative receipt file. Events remain untrusted claims.
func openEventPipe(path string) (*eventLog, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeNamedPipe == 0 {
		return nil, fmt.Errorf("event pipe must be a named pipe")
	}
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return nil, err
	}
	return &eventLog{file: f}, nil
}
