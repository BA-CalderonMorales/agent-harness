package environment

import (
	"context"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/permissions"
)

// Environment defines the interface through which the agent interacts with the system.
type Environment interface {
	// Execute runs a command in the environment's shell.
	Execute(ctx context.Context, cmd string) (string, error)
	// ReadFile retrieves the content of a file from the environment.
	ReadFile(path string) ([]byte, error)
	// WriteFile persists data to a file within the environment.
	WriteFile(path string, data []byte) error
	// Permissions returns the current permission mode.
	Permissions() permissions.Mode
}

// State represents the current status of the environment.
type State struct {
	WorkingDirectory string
	OS               string
	Shell            string
	PermissionMode   permissions.Mode
}
