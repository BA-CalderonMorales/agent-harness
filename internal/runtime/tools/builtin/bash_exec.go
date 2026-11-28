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

const (
	// maxBashBufferBytes caps in-memory command output accumulation well above
	// maxBashOutputChars so runaway commands (yes, huge logs) cannot OOM the
	// process while stdout/stderr pipes continue draining cleanly.
	maxBashBufferBytes  = 256 << 10
	maxBashPendingBytes = 64 << 10
)

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
	var bufferTruncated bool
	stdout := &bashOutputWriter{output: &output, outputMu: &outputMu, truncated: &bufferTruncated, onProgress: onProgress}
	stderr := &bashOutputWriter{output: &output, outputMu: &outputMu, truncated: &bufferTruncated, onProgress: onProgress}

	cmd.Stdout, cmd.Stderr = stdout, stderr

	if err := cmd.Start(); err != nil {
		return tools.ToolResult{Data: "[error starting command: " + err.Error() + "]", IsError: true}, nil
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

	// Truncate command output BEFORE appending the failure/timeout marker so
	// verbose failing commands never lose their exit status or timeout reason.
	result = truncateBashOutput(result)
	if bufferTruncated && !strings.Contains(result, "[Output truncated:") {
		result += "\n\n[Output truncated: exceeded buffer limit]"
	}

	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			result += "\n[command timed out after " + timeout.String() + "]"
		} else {
			result += "\n[exit status: " + err.Error() + "]"
		}
	}

	return tools.ToolResult{Data: result, IsError: err != nil}, nil
}

type bashOutputWriter struct {
	output     *strings.Builder
	outputMu   *sync.Mutex
	truncated  *bool
	onProgress tools.OnProgress
	pending    []byte
}

func (w *bashOutputWriter) appendOutputLocked(s string) {
	if w.output.Len() >= maxBashBufferBytes {
		*w.truncated = true
		return
	}
	remaining := maxBashBufferBytes - w.output.Len()
	if len(s) > remaining {
		w.output.WriteString(s[:remaining])
		*w.truncated = true
		return
	}
	w.output.WriteString(s)
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
		w.appendOutputLocked(line + "\n")
		w.pending = w.pending[newline+1:]
		lines = append(lines, line)
	}
	// Compact pending slice so backing arrays from earlier chunks can be reused
	// and an unterminated multi-megabyte stream cannot grow pending unboundedly.
	if len(w.pending) == 0 {
		w.pending = w.pending[:0]
	} else if len(w.pending) > maxBashPendingBytes {
		w.appendOutputLocked(string(w.pending))
		w.pending = w.pending[:0]
	} else if cap(w.pending) > maxBashPendingBytes && len(w.pending) < maxBashPendingBytes/4 {
		compact := make([]byte, len(w.pending))
		copy(compact, w.pending)
		w.pending = compact
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
	w.appendOutputLocked(tail)
	w.outputMu.Unlock()
	if tail != "" && w.onProgress != nil {
		w.onProgress(tail)
	}
}
