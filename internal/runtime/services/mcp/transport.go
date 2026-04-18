package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type Transport interface {
	Start(ctx context.Context) error
	Send(ctx context.Context, req JSONRPCRequest) (*JSONRPCResponse, error)
	Close() error
}

type stdioTransport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	stderr io.ReadCloser
	mu     sync.Mutex
	env    map[string]string
}

func NewStdioTransport(command string, args []string, env map[string]string) Transport {
	return &stdioTransport{
		cmd: exec.Command(command, args...),
		env: env,
	}
}

func (t *stdioTransport) Start(ctx context.Context) error {
	cmdEnv := os.Environ()
	for k, v := range t.env {
		cmdEnv = append(cmdEnv, k+"="+v)
	}
	t.cmd.Env = cmdEnv

	stdin, err := t.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := t.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	t.stdin = stdin
	t.stdout = bufio.NewReader(stdout)
	t.stderr = stderr

	go drainStderr(stderr)

	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("start command: %w", err)
	}
	return nil
}

func drainStderr(r io.ReadCloser) {
	defer r.Close()
	buf := make([]byte, 64*1024)
	for {
		_, err := r.Read(buf)
		if err != nil {
			return
		}
	}
}

func (t *stdioTransport) Send(ctx context.Context, req JSONRPCRequest) (*JSONRPCResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	type result struct {
		resp *JSONRPCResponse
		err  error
	}

	done := make(chan result, 1)
	go func() {
		data, err := json.Marshal(req)
		if err != nil {
			done <- result{err: fmt.Errorf("marshal request: %w", err)}
			return
		}
		data = append(data, '\n')
		if _, err := t.stdin.Write(data); err != nil {
			done <- result{err: fmt.Errorf("write stdin: %w", err)}
			return
		}
		line, err := t.stdout.ReadString('\n')
		if err != nil {
			done <- result{err: fmt.Errorf("read stdout: %w", err)}
			return
		}
		var resp JSONRPCResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			done <- result{err: fmt.Errorf("unmarshal response: %w", err)}
			return
		}
		done <- result{resp: &resp}
	}()

	select {
	case r := <-done:
		return r.resp, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (t *stdioTransport) Close() error {
	if t.stdin != nil {
		_ = t.stdin.Close()
	}
	if t.cmd == nil || t.cmd.Process == nil {
		return nil
	}

	_ = t.cmd.Process.Signal(syscall.SIGTERM)

	done := make(chan error, 1)
	go func() {
		done <- t.cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		_ = t.cmd.Process.Kill()
		return <-done
	}
}
