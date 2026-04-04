# Command Approval System

The command approval system provides transparency and control over what the agent executes on your system. It implements patterns from production agentic coding tools like Kimi CLI and Claude Code.

## Overview

The approval system has two modes:

### Interactive Mode (Default)

Before executing potentially dangerous commands, the system displays:
- The exact command to be executed
- The tool being used (Shell, Write, Edit)
- A warning if the command is destructive

You can choose:
- **Approve** (a) - Run this command once
- **Approve All** (A) - Always run similar commands (remembers your choice)
- **Reject** (r) - Skip this command
- **Reject + Suggest** (R) - Skip and tell the agent what to do instead

### Yolo Mode

For trusted workflows, yolo mode:
- Auto-approves all commands
- Still shows what commands are executing in the UI
- Lets you see the full execution trace
- Press ESC at any time to cancel

## Configuration

Set your preferred mode in Settings or via configuration:

```json
{
  "execution_mode": "interactive"
}
```

Values: `interactive` | `yolo`

## ESC Key Behavior

The ESC key is context-aware:
- **During command approval** - Rejects the current command
- **During agent execution** - Cancels the entire agent loop
- **Normal mode** - Switches from insert to normal mode (vim-like)

## Implementation

The approval system is implemented in:

- `internal/approval/` - Core types and manager
- `internal/tui/approval_dialog.go` - UI component
- `cmd/agent-harness/main.go` - Integration with agent loop

### Key Types

```go
// ExecutionMode controls approval behavior
type ExecutionMode int
const (
    ModeInteractive ExecutionMode = iota  // Prompt for each command
    ModeYolo                              // Auto-approve with visibility
)

// Decision represents user choice
type Decision int
const (
    DecisionPending     Decision = iota  // Waiting for input
    DecisionApprove                       // Run this once
    DecisionReject                        // Skip this
    DecisionApproveAll                    // Run this and similar
    DecisionRejectAll                     // Skip and suggest alternative
)
```

### Pattern Matching

When you choose "Approve All" or "Reject All", the system remembers the exact command pattern. Future identical commands are automatically approved or rejected without prompting.

This is stored in memory only (not persisted) for security.

## Security Considerations

1. **Destructive commands** are flagged with visual warnings
2. **Pattern memory** only lasts for the session
3. **ESC key** provides immediate cancellation
4. **Command visibility** - You always see what is about to run

## Testing

Run the approval system tests:

```bash
go test ./internal/approval/... -v
```

Key test scenarios:
- Mode switching
- Decision propagation
- Pattern memory
- Context cancellation
- Timeout handling
