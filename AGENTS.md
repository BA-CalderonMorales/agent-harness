# Agent Harness - Development Guide

> Patterns and conventions for working on this codebase.

## Quick Reference

```
cmd/agent-harness/          # CLI entrypoint
internal/
  agent/                    # Core query loop
  tools/builtin/            # Tool implementations
  permissions/              # Permission stack
  llm/                      # LLM client
pkg/bash/, git/, sandbox/   # Infrastructure

# Run locally
cd ~/projects/agent-harness && ./scripts/run-termux.sh

# Or from buckets/usr/bin (workspace-level access)
~/buckets/usr/bin/agent-harness
```

## Key Patterns

### Tool Descriptor Pattern

Tools are structs with function fields, not interfaces:

```go
var MyTool = tools.NewTool(tools.Tool{
    Name: "my_tool",
    Capabilities: tools.CapabilityFlags{
        IsEnabled:         func() bool { return true },
        IsConcurrencySafe: func(input map[string]any) bool { return true },
        InterruptBehavior: func() string { return "cancel" },
    },
    ValidateInput:    func(input map[string]any, ctx tools.Context) tools.ValidationResult { ... },
    CheckPermissions: func(input map[string]any, ctx tools.Context) tools.PermissionDecision { ... },
    Call:             func(input map[string]any, ctx tools.Context, ...) (tools.ToolResult, error) { ... },
    MapResult:        func(result any, toolUseID string) types.ToolResultBlock { ... },
})
```

### Permission Stack (top to bottom)

1. Blanket deny rules
2. Blanket allow rules
3. Always-ask rules
4. Mode transformations
5. Tool-specific `CheckPermissions()`
6. Default: ask

### File Operations

**Cache reads** by `(path, offset, limit, mtime)` - see `internal/fs/cache.go`.

**Stale-write protection** - check `fs.DefaultStaleTracker.CheckStale(path)` before writes.

**Atomic writes** - temp file + rename pattern.

## Security

- UNC paths (`//server`) rejected (prevent NTLM leaks)
- Device paths (`/dev/zero`, `/proc/*/fd/*`) blocked
- Bash uses `exec.LookPath("sh")` for portability

## Termux-Specific

- Build: `go build -o ./build/agent-harness ./cmd/agent-harness`
- /tmp is restricted - use project-local dirs
- Shell at `$PREFIX/bin/sh` (Android-compatible)
- See `docs/os/termux/ui_ux_fixes.md` for detailed UI/UX patches

Skill loaded from: `.agent-harness/skills/termux-mobile-dev/SKILL.md`
- Release workflow: `.agent-harness/skills/release-workflow/SKILL.md`

## Conversation Flow

The agent uses a dual-path system for handling user input:

### Fast Path (Conversational)
Greetings and simple questions get immediate responses without LLM calls:

```go
if agent.IsConversational(input) {
    return app.handleConversationalMessage(input)  // No API call
}
```

Examples: "Hello", "Hi", "What can you do?", "Thanks!"

### Full Path (Task-Based)
Work requests use the full agent loop with tools:

```go
return app.handleTaskMessage(input)  // Full agent loop with tools
```

Examples: "Create a file", "Fix the bug", "Search for TODOs"

See `docs/conversation_flow.md` for full details.

## Workspace Integration (buckets/usr)

For cross-repo workspace access, binaries and scripts are symlinked or placed in:

```
~/buckets/usr/bin/          # Executables in PATH
~/buckets/usr/lib/          # Shared libraries/scripts
~/buckets/usr/share/        # Data files
```

This creates a Unix-like hierarchy at the workspace level for tools that need to be accessible across projects without being tied to a specific repo.

Current entries:
- `~/buckets/usr/bin/agent-harness` - Launcher for agent-harness

## Testing

```bash
go test ./...
go test -race ./...
```

## Build Tags

- `kairos` - Autonomous mode
- `context_collapse` - Advanced context restructuring

## Code Style

- Go 1.21+, standard import grouping
- Wrap errors: `fmt.Errorf("...: %w", err)`
- Exported: `PascalCase`, internal: `camelCase`

## Critical Rules

### Tool Calling Must Work Flawlessly

Tool execution is the core value proposition. Any UX improvements must not regress tool functionality.

**Requirements:**
1. **Tool parsing**: LLM output must be parsed correctly every time
2. **Validation**: Input schema validation must catch errors before execution
3. **Execution**: Tools must execute with proper timeout and cancellation
4. **Feedback**: User must see what tool is running and its status
5. **Recovery**: Failed tools must not crash the session

**Visual Feedback Standards:**
```
→ tool-name: <what it's doing>
  ┌( >_<)┘  <spinner while running>
✓ tool-name: <result summary>
```

**No Regressions Allowed:**
- All existing tools must continue to work
- Permission checks must not be bypassed
- Error handling must be preserved
- Streaming responses must not break

### Visual UX Standards

Based on lessons from Terminal Jarvis ADK and Claude Code:

**Status Indicators (Always Use):**
- `◆` - Acknowledgment / start
- `→` - Action in progress
- `✓` - Success
- `✗` - Error
- `?` - Needs input

**Spinner Animation (Kaomoji Style):**
```
┌( >_<)┘  Frame 1
└( >_<)┐  Frame 2
```

**Never Repeat User Input:**
```
# Bad
User: Can you analyze my projects?
AI: Can you analyze my projects? Let me think...

# Good
◆ Analyzing projects...
→ scanning ~/projects...
  found 18 directories
```

**Tool Execution Flow:**
```
◆ Starting: brief description

→ read: loading file.go
  ┌( >_<)┘  reading...

✓ read: 145 lines loaded
```

### Zero Emojis Policy

**NO EMOJIS** in any root-level `.md` files or documentation. This is non-negotiable.

- README.md: Plain text only
- AGENTS.md: Plain text only
- docs/*.md: Plain text only

**Why**: Professional documentation should not rely on pictographic characters. Use words, not symbols.

**Before**: `### 🔐 Secure Credential Storage`
**After**: `### Secure Credential Storage`

### Section Titles Over Horizontal Rules

**NO HORIZONTAL RULES** (`---`) as section separators in markdown files. Use proper heading hierarchy instead.

- H1 (`#`) for document title
- H2 (`##`) for major sections
- H3 (`###`) for subsections
- H4 (`####`) if needed for deeper nesting

**Why**: Horizontal rules are visual clutter. Proper heading hierarchy creates structure and feeds TOC generation.

**Before**:
```markdown
## Features

### Feature A
Content...

---

### Feature B
Content...
```

**After**:
```markdown
## Features

### Feature A
Content...

### Feature B
Content...
```

### Lowercase Filenames

**ALL LOWERCASE** filenames for all documentation files in the repo. The only exceptions are the root `README.md` and `AGENTS.md` which follow GitHub convention.

- `docs/install.md` not `docs/INSTALL.md`
- `docs/parity.md` not `docs/PARITY.md`
- `docs/usage.md` not `docs/USAGE.md`

**Why**: Consistency. Lowercase is easier to type and avoids case-sensitivity issues across platforms.

## Docs

- `docs/architecture.md` - System design
- `docs/edgecases.md` - Edge cases and quirks
- `docs/termux_edge_cases.md` - Termux portability notes (legacy)
- `docs/services-features.md` - Gated capabilities
- `docs/os/termux/ui_ux_fixes.md` - Termux UI/UX implementation details
