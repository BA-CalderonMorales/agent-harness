package builtin

import (
	"bytes"
	"context"
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
	configureCommandProcess(cmd)
	cmd.Cancel = func() error {
		return cancelCommandProcess(cmd)
	}

	// Give os/exec writers rather than exposing its pipes. Wait joins the
	// internal copy goroutines for non-*os.File writers, so all output is
	// drained before the command is considered complete.
	var output strings.Builder
	var outputMu sync.Mutex
	stdout := &bashOutputWriter{output: &output, outputMu: &outputMu, onProgress: onProgress}
	stderr := &bashOutputWriter{output: &output, outputMu: &outputMu, onProgress: onProgress}

	cmd.Stdout, cmd.Stderr = stdout, stderr

	if err := cmd.Start(); err != nil {
		return tools.ToolResult{Data: "[error starting command: " + err.Error() + "]"}, nil
	}

	// Report start
	if onProgress != nil {
		onProgress("running: " + cmdStr)
	}

	// Wait also joins the stdout/stderr copy goroutines. On Unix, cancellation
	// kills the command's process group, including descendants holding pipes.
	err := cmd.Wait()
	stdout.flush()
	stderr.flush()

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

type bashOutputWriter struct {
	output     *strings.Builder
	outputMu   *sync.Mutex
	onProgress tools.OnProgress
	pending    []byte
}

func (w *bashOutputWriter) Write(p []byte) (int, error) {
	w.outputMu.Lock()
	w.pending = append(w.pending, p...)
	var lines []string
	for {
		newline := bytes.IndexByte(w.pending, '\n')
		if newline < 0 {
			break
		}
		line := strings.TrimSuffix(string(w.pending[:newline]), "\r")
		w.output.WriteString(line + "\n")
		w.pending = w.pending[newline+1:]
		lines = append(lines, line)
	}
	w.outputMu.Unlock()
	if w.onProgress != nil {
		for _, line := range lines {
			w.onProgress(line)
		}
	}
	return len(p), nil
}

// flush emits an unterminated final line after Wait has joined the writers.
func (w *bashOutputWriter) flush() {
	w.outputMu.Lock()
	tail := string(w.pending)
	w.pending = nil
	w.output.WriteString(tail)
	w.outputMu.Unlock()
	if tail != "" && w.onProgress != nil {
		w.onProgress(tail)
	}
}
