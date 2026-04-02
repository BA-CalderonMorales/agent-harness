package tasks

import (
	"fmt"
	"time"
)

// Type identifies the kind of task.
type Type string

const (
	TypeLocalShell   Type = "local_bash"
	TypeLocalAgent   Type = "local_agent"
	TypeRemoteAgent  Type = "remote_agent"
	TypeInProcess    Type = "in_process_teammate"
	TypeDream        Type = "dream"
	TypeWorkflow     Type = "local_workflow"
	TypeMonitorMCP   Type = "monitor_mcp"
)

// Status is the lifecycle state of a task.
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusKilled    Status = "killed"
)

// IsTerminal returns true if the task will not transition further.
func IsTerminal(s Status) bool {
	return s == StatusCompleted || s == StatusFailed || s == StatusKilled
}

// Handle is a reference to a running task.
type Handle struct {
	TaskID  string
	Cleanup func()
}

// StateBase is common to all task implementations.
type StateBase struct {
	ID           string
	Type         Type
	Status       Status
	Description  string
	ToolUseID    string
	StartTime    int64
	EndTime      int64
	TotalPausedMs int64
	OutputFile   string
	OutputOffset int64
	Notified     bool
	Events       []Event
}

// Event captures a discrete event in the task's history.
type Event struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"` // "thought", "tool_call", "observation", "error"
	Content   string    `json:"content"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// AddEvent records a new event into the task's state.
func (s *StateBase) AddEvent(eventType, content string, metadata map[string]any) {
	s.Events = append(s.Events, Event{
		Timestamp: time.Now(),
		Type:      eventType,
		Content:   content,
		Metadata:  metadata,
	})
}

// Task is the interface for all task kinds.
// Mirrors Claude Code's Task pattern: each task type knows how to spawn and kill.
type Task interface {
	Name() string
	Type() Type
	Kill(taskID string) error
}

// Registry holds all known task implementations.
type Registry struct {
	tasks map[Type]Task
}

// NewRegistry creates an empty task registry.
func NewRegistry() *Registry {
	return &Registry{tasks: make(map[Type]Task)}
}

// Register adds a task implementation.
func (r *Registry) Register(t Task) {
	r.tasks[t.Type()] = t
}

// Get looks up a task by type.
func (r *Registry) Get(ty Type) (Task, bool) {
	t, ok := r.tasks[ty]
	return t, ok
}

// GenerateTaskID creates a random task ID with a type prefix.
// Prefixes: b=bash, a=agent, r=remote, t=team, d=dream, w=workflow, m=monitor
func GenerateTaskID(ty Type) string {
	prefixes := map[Type]string{
		TypeLocalShell:  "b",
		TypeLocalAgent:  "a",
		TypeRemoteAgent: "r",
		TypeInProcess:   "t",
		TypeDream:       "d",
		TypeWorkflow:    "w",
		TypeMonitorMCP:  "m",
	}
	p := prefixes[ty]
	if p == "" {
		p = "x"
	}
	// Use nanosecond timestamp + random suffix for uniqueness
	return fmt.Sprintf("%s%d", p, time.Now().UnixNano())
}

// CreateStateBase initializes common task state.
func CreateStateBase(id string, ty Type, description string, toolUseID string) StateBase {
	return StateBase{
		ID:          id,
		Type:        ty,
		Status:      StatusPending,
		Description: description,
		ToolUseID:   toolUseID,
		StartTime:   time.Now().UnixMilli(),
		OutputFile:  fmt.Sprintf("/tmp/agent-harness/%s.log", id),
		OutputOffset: 0,
		Notified:    false,
	}
}
