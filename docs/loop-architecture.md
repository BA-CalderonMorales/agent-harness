# Loop Architecture

The agent loop is decomposed into focused interfaces that can be implemented
independently by "buckets" - domain-specific loop implementations. The
standard agent control flow (`internal/agent/loop.go:queryLoop()`):

```
while not done:
    1. Call LLM with current message context
    2. If text-only response -> done
    3. If tool calls -> execute, add results, continue
    4. If max turns exceeded -> error
```

Max turns: 10 (configurable). Tool execution supports batching by
concurrency safety.

## Core Interfaces (`internal/loop/`)

| Interface | Purpose | File |
|-----------|---------|------|
| `LoopBase` | Fundamental contract for all buckets | `base.go` |
| `LoopConfig` | Unified configuration | `config.go` |
| `LoopError` | Structured error handling | `error.go` |
| `LoopResults` | Result aggregation | `results.go` |
| `LoopSystemPrompts` | Prompt composition | `prompts.go` |
| `LoopExecute` | Execution strategies | `execute.go` |
| `LoopTool` | Tool management | `tool.go` |

## Bucket Implementations (`internal/loop/buckets/`)

| Type | Constructor | Handles | Capabilities |
|------|-------------|---------|--------------|
| `FileSystemBucket` | `FileSystem(basePath)` | read, write, glob, edit | Concurrency-safe, destructive |
| `ShellBucket` | `Shell(basePath)` | bash, execute_command | Serial, destructive |
| `SearchBucket` | `Search(basePath)` | grep, search, find | Concurrency-safe, read-only |
| `GitBucket` | `Git(basePath)` | git_status, git_diff, git_commit | Serial, destructive |
| `PlanBucket` | `Plan()` | enter_plan_mode, exit_plan_mode | - |
| `TranscriptBucket` | `Transcript()` | search_transcript | Read-only |
| `UIBucket` | `UI(exportDir, notebookDir)` | ask, todo, export | Interactive |
| `WebBucket` | `Web()` | webfetch, web_search | Network, read-only |
| `CodeBucket` | `Code(basePath)` | lint, format, analyze | Read-only |
| `TestBucket` | `Test(basePath)` | run_tests | Destructive |
| `AgentBucket` | `Agent(basePath, client)` | spawn sub-agents | Recursive |

## Defaults System (`internal/loop/buckets/defaults/`)

All hardcoded configuration centralized:

```go
// defaults/shell.go
const ShellDefaultTimeout = 60 * time.Second
var ShellBlockedCommands = []string{"rm -rf /", ":(){ :|:& };:"}
```

Buckets import and use - no magic numbers in implementations.

## Creating an Orchestrator

```go
// Using factory presets
orch := loop.CreateFromPreset(loop.PresetStandard, basePath, llmClient)
orth := loop.CreateFromPreset(loop.PresetFast, basePath, llmClient)
orth := loop.CreateFromPreset(loop.PresetSafe, basePath, llmClient)

// Using factory with config
factory := loop.NewFactory(basePath, llmClient).
    WithConfig(loop.FastConfig())
orth := factory.CreateStandard()

// Using builder for custom setup
orch := factory.NewBuilder().
    WithFileSystem(func(fs *buckets.FileSystemBucket) {
        fs.WithBlockedPaths("/etc", "/usr")
    }).
    WithShell(func(sh *buckets.ShellBucket) {
        sh.WithTimeout(30).WithoutApproval()
    }).
    WithSearch().
    Build()

// Direct construction
orch := loop.Orchestration(config, client,
    buckets.FileSystem(basePath),
    buckets.Shell(basePath),
    buckets.Search(basePath),
)
```

## Implementing a Custom Bucket

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

## Naming Conventions

### Go Restriction: No Type/Function Name Collision

Go does not allow a type and function to share the same name in one package:

```go
type Orchestrator struct {}  // Type declaration
func Orchestrator() {}       // ERROR: redeclared (same name)
func NewOrchestrator() {}    // OK: different name
```

### Bucket Suffix Pattern

To achieve readable constructors without `New*` prefixes, we use:

- **Type name**: `<Domain>Bucket` (e.g., `FileSystemBucket`)
- **Constructor**: `<Domain>()` (e.g., `FileSystem()`)

```go
// Type declaration
type FileSystemBucket struct { ... }

// Constructor - readable, no "New" prefix
func FileSystem(basePath string) *FileSystemBucket {
    return &FileSystemBucket{...}
}

// Usage
fs := buckets.FileSystem("/path")
```

This pattern applies to all buckets:

| Type | Constructor |
|------|-------------|
| `OrchestrationBucket` | `loop.Orchestration(...)` |
| `FileSystemBucket` | `buckets.FileSystem(...)` |
| `ShellBucket` | `buckets.Shell(...)` |
| `SearchBucket` | `buckets.Search(...)` |
