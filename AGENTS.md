# AGENTS.md - Agent Harness Development Guide

> Domain-specific constitution for this repository.
>
> This file extends [PHILOSOPHY.md](./PHILOSOPHY.md) with project-specific conventions.
>
> **Quick Start**: See patterns and conventions for working on this codebase below.

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

## Working Rules

- If a prompt would require a major architectural deviation, stop and explain before proceeding.
- Keep changes and milestones separated into distinct commits.
- After each change or milestone, commit and push before starting the next one.
- Do not bundle unrelated work from different prompts into the same commit.

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

The agent uses a unified agentic loop for ALL user input. There is no "fast path" that bypasses the LLM.

### Agentic Model (Unified Path)
EVERY user message enters the full agent loop (`agent.Loop.Query`). The LLM decides how to respond:

```go
// ALL messages go through the agent loop
app.handleAgentLoopAsync(input, tuiApp)  // Full LLM + tools
```

For a greeting like "Hello":
1. User message added to session
2. LLM receives: system prompt + user message
3. LLM decides: "This is just a greeting → respond warmly, no tools"
4. Assistant response streamed back to user

For a task like "List all Go files":
1. User message added to session
2. LLM receives: system prompt + user message
3. LLM decides: "They want to list files → use bash tool"
4. Tool call executed, result returned to LLM
5. Final response streamed to user

### Why This Matters

**Before (Fast Path):** Code decided if input was "conversational" and gave canned responses. The LLM never saw greetings, so it had no context of the conversation history.

**After (Agentic):** The LLM sees every message. It maintains full conversation context. It decides when to use tools. This feels truly agentic, not like a chatbot with scripted responses.

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

## Response Time Tracking

The TUI includes real-time response time tracking inspired by lumina-bot:

### Timer Display (During Generation)
```
◆ <user input>

┌( >_<)┘ thinking (2.3s)          <- Live elapsed time
┌( >_<)┘ streaming (4.1s | 12 chunks)  <- With chunk count
```

### Response Time (After Completion)
```
Agent 14:32:05 (6.2s)             <- Total response time shown
<response content>
```

### Implementation Details
- Timer starts when `AgentStartMsg` is received
- Updates every 100ms via `tea.Tick` command
- Tracks chunk count for streaming responses
- Elapsed time captured in `ChatMessage.ResponseTime`
- Displayed in assistant message header after completion

### State Management
```go
type ChatModel struct {
    startTime    time.Time     // When request started
    elapsed      time.Duration // Current elapsed time
    timerRunning bool          // Is timer active
    chunkCount   int           // Number of chunks received
}
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

## Release Tag Retention

**NEVER DELETE RELEASE TAGS**. Tags are part of the project's permanent history.

### Why Keep All Tags

1. **Audit trail**: Tags mark when specific versions were released
2. **Bug reports**: Users may reference old versions when reporting issues
3. **Bisecting**: Developers need tags to find when regressions were introduced
4. **History**: Deleted tags create gaps in the release timeline

### What to Do Instead

If a release has issues:
1. Fix the issue on `develop`
2. Bump the version
3. Create a new tag with an incremented version number
4. Document the fix in the release notes

### Example

```bash
# Bad - deleting a tag
git push --delete origin v0.0.28
git tag -d v0.0.28

# Good - keeping history and releasing fix
# v0.0.28 had a bug
# Fix it on develop...
git tag -a v0.0.29 -m "Fix bug from v0.0.28"
git push origin v0.0.29
```

Both v0.0.28 and v0.0.29 exist in history. Users can see the progression.
