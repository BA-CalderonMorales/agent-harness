# AGENTS.md - Agent Harness

## Quick Reference

- **Source**: `cmd/agent-harness/main.go`
- **Run**: `./scripts/run-termux.sh` or `~/buckets/usr/bin/agent-harness`
- **Persona**: `/persona <name>` — switch behavior mode (developer|designer|pm|scientist|explorer)
- **Audit**: `/audit` — show recent tool activity
- **Home tab**: `h` — jump to dashboard, `c` — jump to chat
- **Local LLM**: `agent-harness.yml` targets Ornith-1.0 GGUF through a local `llama.cpp` OpenAI-compatible endpoint
- **Fallback Local LLM**: `./scripts/ah-fast.sh` for small Ollama smoke tests
- **Prune Branches**: `./scripts/prune-branches.sh` (or `--dry-run`)
- **Commit from TUI**: `/commit <message>` — stages all changes and commits

## Recent DX Improvements

- **Persona system**: 5 functional personas (developer, designer, pm, scientist, explorer) adapt prompts, tools, and UI
- **Home dashboard**: project overview, quick actions, recent sessions, contextual hints
- **Audit logging**: append-only log of tool executions and approvals at `~/.agent-harness/audit/`
- **Sandbox preview**: approval dialog shows diff preview for write/edit and risk level for bash
- **Granular permissions**: individual toggles for read/write/delete/execute with mode presets
- **Rich startup context**: system prompt auto-includes git status, recent commits, project file tree
- **Session auto-resume**: loads the most recent session on startup for continuity
- **Tool output truncation**: bash and read outputs are silently capped to protect context budget
- **Loop auto-compaction**: old messages are trimmed automatically when approaching token limits
- **Edit tool**: supports `replace_all` and gives actionable errors on mismatch

## Core Agent Loop

Standard agent control flow (`internal/agent/loop.go:queryLoop()`):

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

## Key Patterns

- **Bucket Suffix Pattern**: Types end with `Bucket`, constructors use base name
- **Bucket Architecture**: Domain-specific LoopBase implementations hide internals
- **Tool Descriptor Pattern**: Structs with function fields, not interfaces
- **Permission Stack**: deny → allow → ask → mode transforms → tool-specific checks
- **File Operations**: cache by (path, offset, limit, mtime), stale-write protection, atomic writes

## Security

- UNC paths rejected (prevent NTLM leaks)
- Device paths blocked
- Bash uses `exec.LookPath("sh")` for portability
- Each bucket validates inputs before execution
- Shell bucket has whitelist/blacklist pattern matching
- Audit logging: all tool executions logged to `~/.agent-harness/audit/YYYY-MM-DD.log`
- Sandbox preview: approval dialog shows file diff preview and bash risk assessment
- Granular permissions: read/write/delete/execute toggles independent of mode presets

## Termux

- Build: `go build -o ./build/agent-harness ./cmd/agent-harness`
- Use project-local dirs (not /tmp)
- Shell at `$PREFIX/bin/sh`

## Environment Variables

- `AH_PROVIDER`: local, ollama, openrouter, openai, anthropic
- `AH_RUNTIME`: llama.cpp, ollama, or another local runtime label
- `AH_MODEL`: model identifier
- `AH_MODEL_PATH`: local GGUF path when using a local runtime
- `AH_ENDPOINT_URL`: OpenAI-compatible API base URL
- `AH_CONTEXT_LENGTH`: model context window
- `AH_TEMPERATURE`: sampling temperature
- `AH_MAX_TOKENS`: maximum response tokens
- `AH_WORKSPACE_PATH`: repository workspace path
- `AH_LOCAL_SERVER_COMMAND`: command users should run for the local server
- `AH_API_KEY`: API key (not needed for local or ollama)
- `AH_PERMISSION_MODE`: read-only, workspace-write, danger-full-access
- `AH_PERSONA`: default persona (developer, designer, pm, scientist, explorer)
- `OLLAMA_HOST`: Ollama server URL for the fallback Ollama scripts

## Testing

- `go test ./...`
- `go test -race ./...`

## Critical Rules

- Zero emojis in root-level .md files
- Lowercase filenames (except README.md, AGENTS.md)
- No horizontal rules as section separators
- Tool calling must work flawlessly - no regressions
- Follow Bucket Suffix naming pattern for new buckets

## Branch Protection

Protected branches: `main`, `develop`

### Local Protection

```bash
# Safe deletion (checks protection)
git del <branch>          # Delete merged branch
git del-force <branch>    # Force delete

# Prune all merged branches
./scripts/prune-branches.sh --dry-run   # Preview
./scripts/prune-branches.sh             # Execute
```

### Remote Protection

Configure in GitHub: Settings → Branches → Add rule
- Pattern: `main` and `develop`
- Enable: "Restrict deletions"
- Optional: Require PR reviews, status checks

See `docs/branch-protection.md` for full setup.

## Working Rules

- Stop and explain before major architectural changes
- One change per commit, commit before starting next
- Conventional commits: `type(scope): description`

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **agent-harness** (5221 symbols, 15963 relationships, 300 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

> Index stale? Run `node .gitnexus/run.cjs analyze` from the project root — it auto-selects an available runner. No `.gitnexus/run.cjs` yet? `npx gitnexus analyze` (npm 11 crash → `npm i -g gitnexus`; #1939).

## Always Do

- **MUST run impact analysis before editing any symbol.** Before modifying a function, class, or method, run `impact({target: "symbolName", direction: "upstream"})` and report the blast radius (direct callers, affected processes, risk level) to the user.
- **MUST run `detect_changes()` before committing** to verify your changes only affect expected symbols and execution flows. For regression review, compare against the default branch: `detect_changes({scope: "compare", base_ref: "main"})`.
- **MUST warn the user** if impact analysis returns HIGH or CRITICAL risk before proceeding with edits.
- When exploring unfamiliar code, use `query({search_query: "concept"})` to find execution flows instead of grepping. It returns process-grouped results ranked by relevance.
- When you need full context on a specific symbol — callers, callees, which execution flows it participates in — use `context({name: "symbolName"})`.
- For security review, `explain({target: "fileOrSymbol"})` lists taint findings (source→sink flows; needs `analyze --pdg`).

## Never Do

- NEVER edit a function, class, or method without first running `impact` on it.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis.
- NEVER rename symbols with find-and-replace — use `rename` which understands the call graph.
- NEVER commit changes without running `detect_changes()` to check affected scope.

## Resources

| Resource | Use for |
|----------|---------|
| `gitnexus://repo/agent-harness/context` | Codebase overview, check index freshness |
| `gitnexus://repo/agent-harness/clusters` | All functional areas |
| `gitnexus://repo/agent-harness/processes` | All execution flows |
| `gitnexus://repo/agent-harness/process/{name}` | Step-by-step execution trace |

## CLI

| Task | Read this skill file |
|------|---------------------|
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus/gitnexus-cli/SKILL.md` |

<!-- gitnexus:end -->
