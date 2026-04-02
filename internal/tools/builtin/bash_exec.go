package builtin

import (
	"context"
	"os/exec"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/tools"
)

// shellPath caches the shell path lookup
var shellPath string

func init() {
	// Try to find bash first, fallback to sh
	if path, err := exec.LookPath("bash"); err == nil {
		shellPath = path
	} else if path, err := exec.LookPath("sh"); err == nil {
		shellPath = path
	}
}

func runBashCommand(ctx context.Context, cmdStr string, timeoutMs int, onProgress tools.OnProgress) (tools.ToolResult, error) {
	timeout := time.Duration(timeoutMs) * time.Millisecond
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Use cached shell path or fallback to "sh"
	shell := shellPath
	if shell == "" {
		shell = "sh"
	}

	cmd := exec.CommandContext(execCtx, shell, "-c", cmdStr)

	output, err := cmd.CombinedOutput()
	result := string(output)
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			result += "\n[command timed out after " + timeout.String() + "]"
		} else {
			result += "\n[error: " + err.Error() + "]"
		}
	}

	return tools.ToolResult{Data: result}, nil
}
