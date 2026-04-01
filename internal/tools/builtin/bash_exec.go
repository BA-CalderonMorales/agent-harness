package builtin

import (
	"context"
	"os/exec"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/tools"
)

func runBashCommand(ctx context.Context, cmdStr string, timeoutMs int, onProgress tools.OnProgress) (tools.ToolResult, error) {
	timeout := time.Duration(timeoutMs) * time.Millisecond
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "sh", "-c", cmdStr)

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
