package builtin

import (
	"bufio"
	"context"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
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

	// Set up pipes for real-time progress
	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return tools.ToolResult{Data: "[error starting command: " + err.Error() + "]"}, nil
	}

	// Report start
	if onProgress != nil {
		onProgress("running: " + cmdStr)
	}

	// Accumulate output
	var output strings.Builder
	var wg sync.WaitGroup
	wg.Add(2)

	processPipe := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			output.WriteString(line + "\n")
			if onProgress != nil {
				onProgress(line)
			}
		}
	}

	go processPipe(stdoutPipe)
	go processPipe(stderrPipe)

	err := cmd.Wait()
	wg.Wait()

	result := output.String()
	if result == "" {
		result = " "
	}

	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			result += "\n[command timed out after " + timeout.String() + "]"
		} else {
			result += "\n[exit status: " + err.Error() + "]"
		}
	}

	result = truncateBashOutput(result)

	return tools.ToolResult{Data: result}, nil
}
