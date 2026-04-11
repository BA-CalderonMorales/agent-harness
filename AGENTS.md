# AGENTS.md - Agent Harness

## Quick Reference

- **Source**: `cmd/agent-harness/main.go`
- **Run**: `./scripts/run-termux.sh` or `~/buckets/usr/bin/agent-harness`
- **Local LLM**: `./scripts/ah-fast.sh` (gemma4:2b) or `./scripts/ah-local.sh` (gemma4:4b)

## Cross-Repo

- Related: terminal-jarvis (Rust ADK), lumina-bot (Go gateway), claude-termux (JS CLI)
- Shared commands: `harness-status`, `sync-philosophy`

## Core Agent Loop

All harnesses share identical control flow (`internal/agent/loop.go:queryLoop()`):

```
while not done:
    1. Call LLM with current message context
    2. If text-only response → done
    3. If tool calls → execute, add results, continue
    4. If max turns exceeded → error
```

Max turns: 10 (configurable). Tool execution supports batching by concurrency safety.

## Modular Loop Architecture (New)

The loop is decomposed into focused interfaces that can be implemented independently by "buckets" - domain-specific loop implementations.

### Core Interfaces (`internal/loop/`)

| Interface | Purpose | File |
|-----------|---------|------|
| `LoopBase` | Fundamental contract for all buckets | `base.go` |
| `LoopConfig` | Unified configuration | `config.go` |
| `LoopError` | Structured error handling | `error.go` |
| `LoopResults` | Result aggregation | `results.go` |
| `LoopSystemPrompts` | Prompt composition | `prompts.go` |
| `LoopExecute` | Execution strategies | `execute.go` |
| `LoopTool` | Tool management | `tool.go` |

### Bucket Implementations (`internal/loop/buckets/`)

| Bucket | Handles | Capabilities |
|--------|---------|--------------|
| `LoopFileSystem` | read, write, glob, edit | Concurrency-safe, destructive |
| `LoopShell` | bash, execute_command | Serial, destructive |
| `LoopSearch` | grep, search, find, websearch | Concurrency-safe, read-only |
| `LoopGit` | git_status, git_diff, git_log, git_commit | Approval for destructive |
| `LoopUI` | ask, todo, export, notebook, rewind | Max items, validation |
| `LoopAgent` | agent, sub_agent, delegate | Max depth (5), isolated context |
| `LoopPlan` | enter_plan_mode, exit_plan_mode | Step limits (20), approval gates |
| `LoopTranscript` | search_transcript, summarize | Stop words, topic extraction |
| `LoopWeb` | webfetch, web_search | Blocked hosts, scheme whitelist |
| `LoopCode` | lint, format, analyze_code | Language auto-detect |
| `LoopTest` | run_tests, test | Timeout, parallel limits |

### Defaults System (`internal/loop/buckets/defaults/`)

All hardcoded configuration centralized:

```go
// defaults/shell.go
const ShellDefaultTimeout = 60 * time.Second
var ShellBlockedCommands = []string{"rm -rf /", ":(){ :|:& };:"}
```

Buckets import and use - no magic numbers in implementations.

### Creating an Orchestrator

```go
// Using factory presets
factory := setup.NewFactory(basePath, client)
orth := factory.CreateStandard()   // All buckets
orth := factory.CreateSafe()       // Read-only
orth := factory.CreateFast()       // Speed optimized
orth := factory.CreateRobust()     // Full features

// Using builder for custom setup
orth := factory.NewBuilder().
    WithFileSystem(func(fs *buckets.LoopFileSystem) {
        fs.WithBlockedPaths("/etc", "/usr")
    }).
    WithShell(func(sh *buckets.LoopShell) {
        sh.WithTimeout(30 * time.Second).WithoutApproval()
    }).
    WithSearch().
    Build()
```

### Implementing a Custom Bucket

```go
type MyBucket struct{}

func (b *MyBucket) Name() string { return "mybucket" }

func (b *MyBucket) CanHandle(toolName string, input map[string]any) bool {
    return toolName == "mytool"
}

func (b *MyBucket) Execute(ctx loop.ExecutionContext) loop.LoopResult {
    return loop.LoopResult{Success: true, Data: "result"}
}

func (b *MyBucket) Capabilities() loop.BucketCapabilities {
    return loop.BucketCapabilities{
        Category: "custom",
        ToolNames: []string{"mytool"},
        IsConcurrencySafe: true,
    }
}

// Compile-time check
var _ loop.LoopBase = (*MyBucket)(nil)
```

Then register:
```go
orch.RegisterBucket(&MyBucket{})
```

## Key Patterns

- **Bucket Pattern**: Domain-specific LoopBase implementations hide internals
- **Tool Descriptor Pattern**: Structs with function fields, not interfaces
- **Permission Stack**: deny → allow → ask → mode transforms → tool-specific checks
- **File Operations**: cache by (path, offset, limit, mtime), stale-write protection, atomic writes

## Security

- UNC paths rejected (prevent NTLM leaks)
- Device paths blocked
- Bash uses `exec.LookPath("sh")` for portability
- Each bucket validates inputs before execution
- Shell bucket has whitelist/blacklist pattern matching

## Termux

- Build: `go build -o ./build/agent-harness ./cmd/agent-harness`
- Use project-local dirs (not /tmp)
- Shell at `$PREFIX/bin/sh`

## Environment Variables

- `AH_PROVIDER`: openrouter, openai, anthropic, ollama
- `AH_MODEL`: model identifier
- `AH_API_KEY`: API key (not needed for ollama)
- `OLLAMA_HOST`: Ollama server URL (default: http://localhost:11434)

## Testing

- `go test ./...`
- `go test -race ./...`

## Critical Rules

- Zero emojis in root-level .md files
- Lowercase filenames (except README.md, AGENTS.md)
- No horizontal rules as section separators
- Tool calling must work flawlessly - no regressions

## Working Rules

- Stop and explain before major architectural changes
- One change per commit, commit before starting next
- Conventional commits: `type(scope): description`
