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
- TUI mode may have mobile keyboard issues - prefer CLI

Skill loaded from: `.agent-harness/skills/termux-mobile-dev/SKILL.md`

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

## Docs

- `docs/architecture.md` - System design
- `docs/edgecases.md` - Edge cases and quirks
- `docs/TERMUX_EDGE_CASES.md` - Termux portability notes
- `docs/services-features.md` - Gated capabilities
