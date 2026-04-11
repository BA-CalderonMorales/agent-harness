package environment

import (
	"context"
	"os"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/permissions"
	"github.com/BA-CalderonMorales/agent-harness/pkg/bash"
)

// LocalEnvironment implements the Environment interface for the local host.
type LocalEnvironment struct {
	cwd     string
	permCtx permissions.Context
}

// NewLocalEnvironment creates a new local environment.
func NewLocalEnvironment(cwd string, permCtx permissions.Context) *LocalEnvironment {
	return &LocalEnvironment{
		cwd:     cwd,
		permCtx: permCtx,
	}
}

// Execute runs a command with a default timeout of 30 seconds.
func (l *LocalEnvironment) Execute(ctx context.Context, cmd string) (string, error) {
	// 1. In a production environment, we would call permissions.Evaluate(tool, input, l.permCtx)
	// For this orchestration layer, we provide the execution primitive.
	res, err := bash.Run(ctx, l.cwd, cmd, 30*time.Second)
	if err != nil {
		return res.Combined, err
	}
	return res.Combined, nil
}

// ReadFile reads the content of a file from the host.
func (l *LocalEnvironment) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// WriteFile writes the content to a file on the host.
func (l *LocalEnvironment) WriteFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

// Permissions returns the current permission mode.
func (l *LocalEnvironment) Permissions() permissions.Mode {
	return l.permCtx.Mode
}
