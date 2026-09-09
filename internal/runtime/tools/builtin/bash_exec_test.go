package builtin

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestRunBashCommandTimeoutDoesNotWaitForDescendantPipeHolder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix process groups are not available on Windows")
	}

	start := time.Now()
	result, err := runBashCommand(context.Background(), "sleep 5 & child=$!; printf 'child=%s\\n' \"$child\"; wait", 20, nil)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("runBashCommand() error = %v", err)
	}
	if elapsed >= 250*time.Millisecond {
		t.Fatalf("runBashCommand() took %v after timeout, want it to return promptly", elapsed)
	}

	output, ok := result.Data.(string)
	if !ok || !strings.Contains(output, "command timed out") {
		t.Fatalf("result = %#v, want timeout marker", result.Data)
	}

	childPID, err := descendantPID(output)
	if err != nil {
		t.Fatalf("result = %q: %v", output, err)
	}
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && processExists(childPID) {
		time.Sleep(5 * time.Millisecond)
	}
	if processExists(childPID) {
		t.Fatalf("timed-out descendant process %d is still alive", childPID)
	}
}

func TestRunBashCommandCancellationKillsDescendantPipeHolder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix process groups are not available on Windows")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	childPIDCh := make(chan int, 1)
	start := time.Now()
	result, err := runBashCommand(ctx, "sleep 5 & child=$!; printf 'child=%s\\n' \"$child\"; wait", 5000, func(data any) {
		line, ok := data.(string)
		if !ok || !strings.HasPrefix(line, "child=") {
			return
		}
		if childPID, parseErr := descendantPID(line); parseErr == nil {
			childPIDCh <- childPID
			cancel()
		}
	})
	if err != nil {
		t.Fatalf("runBashCommand() error = %v", err)
	}
	if elapsed := time.Since(start); elapsed >= 250*time.Millisecond {
		t.Fatalf("runBashCommand() took %v after cancellation, want it to return promptly", elapsed)
	}
	if output, ok := result.Data.(string); !ok || !strings.Contains(output, "exit status") {
		t.Fatalf("result = %#v, want exit-status marker", result.Data)
	}

	select {
	case childPID := <-childPIDCh:
		deadline := time.Now().Add(500 * time.Millisecond)
		for time.Now().Before(deadline) && processExists(childPID) {
			time.Sleep(5 * time.Millisecond)
		}
		if processExists(childPID) {
			t.Fatalf("canceled descendant process %d is still alive", childPID)
		}
	default:
		t.Fatal("command did not report descendant PID before cancellation")
	}
}

func TestRunBashCommandDrainsLargeInterleavedOutput(t *testing.T) {
	const lines = 4000
	command := "i=1; while [ $i -le " + strconv.Itoa(lines) + " ]; do printf 'stdout-%04d\\n' $i; printf 'stderr-%04d\\n' $i >&2; i=$((i+1)); done"
	var stdoutLines atomic.Int64
	var stderrLines atomic.Int64

	result, err := runBashCommand(context.Background(), command, 5000, func(data any) {
		line, ok := data.(string)
		if !ok {
			return
		}
		switch {
		case strings.HasPrefix(line, "stdout-"):
			stdoutLines.Add(1)
		case strings.HasPrefix(line, "stderr-"):
			stderrLines.Add(1)
		}
	})
	if err != nil {
		t.Fatalf("runBashCommand() error = %v", err)
	}

	output, ok := result.Data.(string)
	if !ok {
		t.Fatalf("result data type = %T, want string", result.Data)
	}
	if got := stdoutLines.Load(); got != lines {
		t.Fatalf("stdout progress lines = %d, want %d", got, lines)
	}
	if got := stderrLines.Load(); got != lines {
		t.Fatalf("stderr progress lines = %d, want %d", got, lines)
	}
	if !strings.Contains(output, "Output truncated") {
		t.Fatalf("output = %q, want truncation marker", output)
	}
}

func descendantPID(output string) (int, error) {
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "child=") {
			return strconv.Atoi(strings.TrimPrefix(line, "child="))
		}
	}
	return 0, fmt.Errorf("missing child PID")
}

func processExists(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if err := process.Signal(os.Signal(syscall.Signal(0))); err == nil {
		return true
	}
	return false
}
