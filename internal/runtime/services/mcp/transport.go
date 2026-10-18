package mcp

import (
	"os"
	"os/exec"
	"sort"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Transport delegates framing, request correlation, cancellation, and process
// shutdown to the official MCP implementation instead of a second RPC stack.
type Transport = sdk.Transport

func NewStdioTransport(command string, args []string, env map[string]string) Transport {
	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		cmd.Env = append(cmd.Env, key+"="+env[key])
	}
	// A nil Stderr discards diagnostics without retaining unbounded or
	// potentially secret logs, keeping them separate from protocol stdout.
	return &sdk.CommandTransport{Command: cmd, TerminateDuration: time.Second}
}
