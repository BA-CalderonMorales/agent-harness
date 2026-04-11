package buckets

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/session/loop"
	"github.com/BA-CalderonMorales/agent-harness/internal/session/loop/buckets/defaults"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// LoopShell handles shell/bash operations.
// It implements LoopBase with strict safety controls.
type ShellBucket struct {
	basePath        string
	allowedCommands []string // Whitelist (empty = allow all non-destructive)
	blockedCommands []string // Blacklist
	blockedPatterns []*regexp.Regexp
	maxTimeout      time.Duration
	maxOutputSize   int
	requireApproval bool
}

// NewLoopShell creates a shell bucket with safe defaults.
func Shell(basePath string) *ShellBucket {
	return &ShellBucket{
		basePath:        basePath,
		blockedCommands: defaults.ShellBlockedCommands,
		blockedPatterns: defaults.ShellBlockedPatterns,
		maxTimeout:      defaults.ShellDefaultTimeout,
		maxOutputSize:   defaults.ShellMaxOutputSize,
		requireApproval: true,
	}
}

// WithTimeout sets the maximum command execution time.
func (sh *ShellBucket) WithTimeout(d time.Duration) *ShellBucket {
	sh.maxTimeout = d
	return sh
}

// WithAllowedCommands restricts to specific commands.
func (sh *ShellBucket) WithAllowedCommands(cmds ...string) *ShellBucket {
	sh.allowedCommands = cmds
	return sh
}

// WithBlockedCommands adds to the blacklist.
func (sh *ShellBucket) WithBlockedCommands(cmds ...string) *ShellBucket {
	sh.blockedCommands = append(sh.blockedCommands, cmds...)
	return sh
}

// WithoutApproval disables approval requirements.
func (sh *ShellBucket) WithoutApproval() *ShellBucket {
	sh.requireApproval = false
	return sh
}

// Name returns the bucket identifier.
func (sh *ShellBucket) Name() string {
	return "shell"
}

// CanHandle determines if this bucket handles the tool.
func (sh *ShellBucket) CanHandle(toolName string, input map[string]any) bool {
	switch toolName {
	case "bash", "shell", "execute_command", "exec":
		return true
	}
	return false
}

// Capabilities describes what this bucket can do.
func (sh *ShellBucket) Capabilities() loop.BucketCapabilities {
	return loop.BucketCapabilities{
		IsConcurrencySafe: false, // Shell commands are serial
		IsReadOnly:        false, // Can modify state
		IsDestructive:     true,  // Can be destructive
		ToolNames:         []string{"bash", "shell", "execute_command", "exec"},
		Category:          "shell",
	}
}

// Execute runs the shell command.
func (sh *ShellBucket) Execute(ctx loop.ExecutionContext) loop.LoopResult {
	command, ok := ctx.Input["command"].(string)
	if !ok || command == "" {
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("invalid_input", "command is required"),
		}
	}

	// Security validation
	if err := sh.validateCommand(command); err != nil {
		return loop.LoopResult{
			Success: false,
			Error:   loop.WrapError("validation_failed", err),
		}
	}

	// Check permission if required
	if sh.requireApproval && ctx.CanUseTool != nil {
		decision, err := ctx.CanUseTool("bash", ctx.Input, tools.Context{})
		if err != nil {
			return loop.LoopResult{
				Success: false,
				Error:   loop.WrapError("permission_check_failed", err),
			}
		}
		if decision.Behavior == "deny" {
			return loop.LoopResult{
				Success: false,
				Error:   loop.NewLoopError("permission_denied", decision.Message),
			}
		}
	}

	// Execute with progress reporting
	if ctx.OnProgress != nil {
		ctx.OnProgress(map[string]any{"status": "executing", "command": command})
	}

	timeout := sh.maxTimeout
	if t, ok := ctx.Input["timeout"].(float64); ok {
		timeout = time.Duration(t) * time.Millisecond
	}

	execCtx, cancel := context.WithTimeout(ctx.Context, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "sh", "-c", command)
	cmd.Dir = sh.basePath

	output, err := cmd.CombinedOutput()

	if ctx.OnProgress != nil {
		ctx.OnProgress(map[string]any{"status": "complete"})
	}

	if err != nil && execCtx.Err() == context.DeadlineExceeded {
		return loop.LoopResult{
			Success:   false,
			Error:     loop.NewLoopError("timeout", fmt.Sprintf("command timed out after %v", timeout)),
			Retryable: true,
		}
	}

	result := string(output)
	if len(result) > sh.maxOutputSize {
		result = result[:sh.maxOutputSize] + "\n[output truncated]"
	}

	isError := err != nil
	if isError {
		result = fmt.Sprintf("Error: %v\n%s", err, result)
	}

	return loop.LoopResult{
		Success: !isError,
		Data:    result,
		Error: func() loop.LoopError {
			if isError {
				return loop.NewLoopError("command_failed", err.Error())
			}
			return loop.LoopError{}
		}(),
		Messages: []types.Message{{
			Role: types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{
				ToolUseID: ctx.ToolUseID,
				Content:   result,
				IsError:   isError,
			}},
		}},
	}
}

// validateCommand checks if a command is allowed.
func (sh *ShellBucket) validateCommand(cmd string) error {
	// Check blocked patterns
	for _, pattern := range sh.blockedPatterns {
		if pattern.MatchString(cmd) {
			return fmt.Errorf("command matches blocked pattern")
		}
	}

	// Check blocked commands
	for _, blocked := range sh.blockedCommands {
		if strings.Contains(cmd, blocked) {
			return fmt.Errorf("command contains blocked pattern: %s", blocked)
		}
	}

	// Check whitelist if defined
	if len(sh.allowedCommands) > 0 {
		allowed := false
		for _, a := range sh.allowedCommands {
			if strings.HasPrefix(cmd, a) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("command not in allowed list")
		}
	}

	return nil
}

// IsDestructiveCommand detects if a command might be destructive.
func IsDestructiveCommand(cmd string) bool {
	for _, pattern := range defaults.ShellDestructivePatterns {
		if strings.Contains(cmd, pattern) {
			return true
		}
	}
	return false
}

// Ensure LoopShell implements LoopBase
var _ loop.LoopBase = (*ShellBucket)(nil)
