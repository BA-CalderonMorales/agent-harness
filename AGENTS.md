# Agent Harness - Development Guide

> This file contains patterns, conventions, and architectural decisions for agents working on the `agent-harness` codebase.

---

## Project Overview

`agent-harness` is a clean-room, pattern-derived agent implementation in Go. It captures the architectural patterns from production agentic coding tools (Claude Code, OpenCode, etc.) without copying their implementation.

### Core Philosophy

1. **Fail Closed**: All safety mechanisms default to restrictive
2. **Streaming First**: Everything is async/generator-based
3. **Descriptor Pattern**: Tools are structs with function fields, not interfaces
4. **Extension Points**: Feature-gated capabilities via build tags

---

## Architecture Quick Reference

```
cmd/agent-harness/          # CLI entrypoint
internal/
  agent/                    # Core query loop + recovery
  tools/                    # Tool descriptor pattern + registry
    builtin/                # Built-in tool implementations
  permissions/              # Permission stack + auto-classifier
  contextmgr/               # Context compaction strategies
  llm/                      # LLM client abstraction
  fs/                       # File cache + stale-write tracker
  tasks/                    # Task lifecycle
  state/                    # In-memory store + persistence
  services/mcp/             # MCP protocol support
pkg/
  bash/, git/, sandbox/     # Infrastructure packages
```

---

## Key Patterns

### 1. Tool Descriptor Pattern

Tools are **descriptor structs** with function fields, not interfaces:

```go
var MyTool = tools.NewTool(tools.Tool{
    Name: "my_tool",
    // Capabilities are functions, not booleans
    Capabilities: tools.CapabilityFlags{
        IsEnabled:         func() bool { return true },
        IsConcurrencySafe: func(input map[string]any) bool { return true },
        InterruptBehavior: func() string { return "cancel" },
    },
    // Lifecycle hooks
    ValidateInput:    func(input map[string]any, ctx tools.Context) tools.ValidationResult { ... },
    CheckPermissions: func(input map[string]any, ctx tools.Context) tools.PermissionDecision { ... },
    Call:             func(input map[string]any, ctx tools.Context, ...) (tools.ToolResult, error) { ... },
    MapResult:        func(result any, toolUseID string) types.ToolResultBlock { ... },
})
```

### 2. Permission Decision Stack

Permission evaluation is layered (top to bottom):
1. Blanket deny rules
2. Blanket allow rules  
3. Always-ask rules
4. Mode transformations (auto, dontAsk, bypass)
5. Tool-specific `CheckPermissions()`
6. Default: ask

### 3. Content Replacement Budget

Aggregate tool result sizes are bounded per turn to prevent context explosion:

```go
// Read/search tools are exempt - their content is essential
budget := tools.GetCurrentBudget()
budget.CanUseResult(toolName, resultSize, toolMaxSize)
```

Tools with `MaxResultSizeChars == 0` or marked exempt are not bounded.

### 4. File Read Cache

File reads are cached by `(path, offset, limit, mtime)` to preserve prompt cache tokens:

```go
// In internal/fs/cache.go
cacheKey := fs.MakeKey(path, offset, limit, info)
if cached, ok := fs.DefaultCache.Get(cacheKey); ok {
    return cached, nil  // Cache hit
}
// ... read file ...
fs.DefaultCache.Set(cacheKey, content)
```

### 5. Stale Write Protection

File edits check if the file was modified since the last read:

```go
// Records read version
fs.DefaultStaleTracker.RecordRead(path, content, info)

// Checks before write - on Windows, falls back to content hash
if err := fs.DefaultStaleTracker.CheckStale(path); err != nil {
    return fmt.Errorf("stale write detected: %w", err)
}
```

### 6. Error Recovery with Withholding

Recoverable errors (`max_output_tokens`, `prompt_too_long`) are withheld from consumers until recovery attempts fail:

```go
// Returns recoverableError instead of yielding immediately
if isMaxOutputTokensError(err) {
    return nil, nil, &recoverableError{err: err, reason: "max_output_tokens"}
}

// Up to 3 recovery attempts with increasing token limits
```

### 7. Interrupt Behavior

Tools declare `InterruptBehavior`: `"cancel"` or `"block"` (default: block)

- **cancel**: Tool stops on user interrupt, results discarded
- **block**: Tool continues running, user message queued

---

## Security Considerations

### UNC Path Rejection

UNC paths (`\\server` or `//server`) are rejected to prevent SMB authentication dialogs and NTLM credential leaks:

```go
if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, "//") {
    return tools.ValidationResult{Valid: false, Message: "UNC paths not supported"}
}
```

### Blocked Device Paths

Reading from `/dev/zero`, `/dev/random`, `/proc/*/fd/*`, etc. is blocked to prevent hangs:

```go
var blockedDevicePaths = map[string]bool{
    "/dev/zero": true, "/dev/random": true, /* ... */
}
```

### Atomic File Writes

File writes use temp-file-then-rename for atomicity:

```go
tempPath := path + ".tmp"
if err := os.WriteFile(tempPath, data, 0644); err != nil {
    return err
}
if err := os.Rename(tempPath, path); err != nil {
    os.Remove(tempPath)
    return err
}
```

---

## Testing

Run tests:
```bash
go test ./...
```

Run with race detector:
```bash
go test -race ./...
```

---

## Build Tags for Features

Use build tags to gate experimental features:

```go
//go:build kairos

package agent

// Kairos mode implementation
```

Current extension points:
- `kairos` - Autonomous agent mode
- `context_collapse` - Advanced context restructuring
- `history_snip` - Aggressive history trimming

---

## Code Style

- **Go version**: 1.21+
- **Imports**: Standard, then external, then internal
- **Error handling**: Wrap with context: `fmt.Errorf("...: %w", err)`
- **Naming**: `PascalCase` for exported, `camelCase` for internal
- **Comments**: Start with capital letter, end with period for exported items

---

## Resources

- Architecture: `docs/architecture.md`
- Edge cases: `docs/edgecases.md`
- Service features: `docs/services-features.md`
