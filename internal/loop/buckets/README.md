# Loop Buckets

Modular, composable tool execution architecture for agent-harness.

## Philosophy

Each bucket is a **domain-specific LoopBase implementation** that:
- Handles ONLY what it knows (filesystem, shell, search, etc.)
- Knows NOTHING about other buckets' internals
- Returns standardized `LoopResult` structures
- Uses `defaults/` package for configuration (no hardcoded strings)

## Architecture

```
┌─────────────────────────────────────────┐
│         LoopOrchestrator                │
│  (coordinates, doesn't dig into buckets) │
└──────────────┬──────────────────────────┘
               │ routes tool calls
    ┌──────────┼──────────┬──────────┐
    ▼          ▼          ▼          ▼
┌───────┐  ┌───────┐  ┌───────┐  ┌───────┐
│  FS   │  │ Shell │  │Search │  │  Git  │
│Bucket │  │Bucket │  │Bucket │  │Bucket │
└───────┘  └───────┘  └───────┘  └───────┘
```

## Available Buckets

| Bucket | Tools | Safety |
|--------|-------|--------|
| `LoopFileSystem` | read, write, edit, glob, ls | blocked paths, max file size |
| `LoopShell` | bash, execute | blocked commands, patterns, timeout |
| `LoopSearch` | grep, find, websearch | max results, excluded dirs |
| `LoopGit` | git_status, git_diff, git_log, git_commit | approval for destructive |
| `LoopUI` | ask, todo, export, notebook, rewind | max items, validation |
| `LoopAgent` | agent (sub-agent spawn) | max depth, isolated context |
| `LoopPlan` | enter_plan_mode, exit_plan_mode | step limits, approval gates |
| `LoopTranscript` | search_transcript, summarize | stop words, topic extraction |
| `LoopWeb` | webfetch, websearch | blocked hosts, scheme whitelist |
| `LoopCode` | lint, format, analyze | language auto-detect |
| `LoopTest` | run_tests | timeout, parallel limits |

## Usage

### Basic

```go
factory := setup.NewFactory("/project", client)
orchestrator := factory.CreateStandard()
```

### Custom

```go
factory := setup.NewFactory("/project", client)
orch := factory.NewBuilder().
    WithFileSystem(func(fs *buckets.LoopFileSystem) {
        fs.WithBlockedPaths("/etc", "/usr")
    }).
    WithShell(func(sh *buckets.LoopShell) {
        sh.WithoutApproval().WithTimeout(30 * time.Second)
    }).
    Build()
```

### Direct

```go
buckets := []loop.LoopBase{
    buckets.NewLoopFileSystem("/project"),
    buckets.NewLoopShell("/project").WithoutApproval(),
    buckets.NewLoopGit("/project"),
}
orchestrator := loop.NewOrchestrator(config, client, buckets...)
```

## Defaults System

All hardcoded values live in `defaults/` package:

```go
// buckets/defaults/shell.go
const (
    ShellDefaultTimeout = 60 * time.Second
    ShellMaxOutputSize  = 1024 * 1024
)

var ShellBlockedCommands = []string{
    "rm -rf /", ":(){ :|:& };:", // fork bomb
}
```

Buckets import and use:

```go
func NewLoopShell(basePath string) *LoopShell {
    return &LoopShell{
        maxTimeout:      defaults.ShellDefaultTimeout,
        maxOutputSize:   defaults.ShellMaxOutputSize,
        blockedCommands: defaults.ShellBlockedCommands,
    }
}
```

## Adding a New Bucket

1. Create `buckets/mydomain.go`
2. Implement `LoopBase` interface:
   - `Name() string`
   - `CanHandle(toolName string, input map[string]any) bool`
   - `Capabilities() BucketCapabilities`
   - `Execute(ctx ExecutionContext) LoopResult`
3. Add defaults to `defaults/mydomain.go`
4. Register in factory presets
5. Add compile check: `var _ loop.LoopBase = (*LoopMyDomain)(nil)`

## Testing

Each bucket is independently testable:

```go
func TestLoopFileSystem(t *testing.T) {
    fs := buckets.NewLoopFileSystem("/tmp/test")
    
    result := fs.Execute(loop.ExecutionContext{
        ToolName: "read",
        Input: map[string]any{"path": "test.txt"},
    })
    
    assert.True(t, result.Success)
}
```

## Security Model

- **Defense in depth**: Each bucket validates its own inputs
- **Fail closed**: Unknown tools return error
- **No direct access**: Buckets only receive `ExecutionContext`
- **Configurable limits**: All timeouts, sizes, counts are configurable
- **Approval gates**: Destructive operations require explicit approval
