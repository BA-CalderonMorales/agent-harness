package bash

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// Result is the output of a shell command.
type Result struct {
	Stdout   string
	Stderr   string
	Combined string
	ExitCode int
}

// Run executes a shell command with optional timeout.
func Run(ctx context.Context, cwd string, command string, timeout time.Duration) (Result, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	if cwd != "" {
		cmd.Dir = cwd
	}

	output, err := cmd.CombinedOutput()
	res := Result{
		Combined: string(output),
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			res.Combined += fmt.Sprintf("\n[timed out after %s]", timeout)
			return res, ctx.Err()
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			res.ExitCode = exitErr.ExitCode()
		} else {
			res.ExitCode = -1
		}
		return res, err
	}

	return res, nil
}

// IsAvailable returns true if the shell is executable.
func IsAvailable() bool {
	_, err := exec.LookPath("sh")
	return err == nil
}
