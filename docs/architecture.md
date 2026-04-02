# Agent Harness Architecture

> This document maps every derived pattern from production agentic coding tools to its implementation in `agent-harness`.

---

## 1. The Core Agent Loop

**Pattern:** `while-true` loop that calls LLM API, checks `stop_reason`, executes tools, appends results, and loops back.

**Location:** `internal/agent/loop.go`

**Key Design Decisions:**
- Uses Go channels for streaming (`<-chan types.StreamEvent`)
- Mutable state is carried in `loopState` between iterations
- Token blocking limit is checked before each API call
- Max output tokens recovery is supported (up to 3 attempts)
- Tool execution can be streaming or batch

---

## 2. Tool Descriptor Pattern

**Pattern:** Tools are not interfaces. They are **descriptor structs** with function fields that define behavior. This allows optional capabilities without interface proliferation.

**Location:** `internal/tools/tool.go`

**Key Design Decisions:**
- `NewTool()` overlays fail-closed defaults onto partial definitions
- Capability flags are functions, not booleans (so they can be input-dependent)
- Lifecycle is explicit: `ValidateInput -> CheckPermissions -> Call -> MapResult`
- `BackfillObservableInput` mutates a shallow clone before permissions see it
- `MapResult` separates internal types from wire format

---

## 3. Tool Registry & Assembly Pipeline

**Pattern:** Tools are assembled through a multi-stage pipeline, not a static map.

**Location:** `internal/tools/tool.go` (`ToolRegistry`)

**Pipeline:**
1. `RegisterBuiltIn()` — static list of builtins
2. `FindToolByName()` — primary name + alias fallback
3. `FilterEnabled()` — runtime gate check
4. MCP tools concatenated *after* built-ins (cache stability)

---

## 4. Streaming Tool Executor

**Pattern:** Tools stream in from the model over time. Executor manages concurrency, sibling abort, and ordered result yielding.

**Location:** `internal/agent/executor.go`

**Key Design Decisions:**
- Concurrency-safe tools run in parallel
- Non-concurrent tools get exclusive access
- **Sibling abort:** Bash errors cancel other Bash subprocesses via `context.WithCancel`
- Results are buffered and emitted in original order
- Each tool has a state machine: `queued -> executing -> completed`

---

## 5. Permission Decision Stack

**Pattern:** Layered, fail-closed permission evaluation with mode-based controls.

**Location:** `internal/permissions/engine.go`, `internal/config/layered.go`

**Stack (top to bottom):**
1. Permission Mode (read-only / workspace-write / danger-full-access)
2. Always-allow / Always-deny lists from config
3. Mode transformations (`auto`, `bypassPermissions`)
4. Tool-specific `CheckPermissions()`
5. Default: ask

**Permission Modes:**
- `read-only`: Only read/search tools allowed (bash/write/edit blocked)
- `workspace-write`: Most tools allowed, dangerous ones require confirmation
- `danger-full-access`: All tools run without confirmation

---

## 6. Auto-Mode Classifier

**Pattern:** A separate safety classifier that races against user input to decide whether auto-approved actions are safe.

**Location:** `internal/permissions/classifier.go`

**Key Design Decisions:**
- Fast path for always-safe tools (`todo_write`, plan mode)
- Fast path for read-only operations
- Conservative default: block unless confident
- In production, makes a separate LLM call with a mini-transcript

---

## 7. Context Compaction (Three-Layer Strategy)

**Pattern:** Three independent strategies for managing context window pressure.

**Location:** `internal/contextmgr/compact.go`, `internal/state/session.go`

**Layers:**
1. **Session Compaction** — removes older messages, preserves recent context
2. **SnipCompact** — removes zombie messages and stale compact boundaries
3. **ContextCollapse** — advanced restructuring (extension point, no-op in base)

**CompactionConfig:**
- `MaxMessages`: Maximum messages before compaction triggers
- `MaxEstimatedTokens`: Token threshold for compaction
- `PreserveRecent`: Always keep this many recent messages

---

## 8. Task System

**Pattern:** Background/local/remote tasks with unified lifecycle and ID generation.

**Location:** `internal/tasks/task.go`

**Key Design Decisions:**
- Task IDs use type prefixes (`b=bash`, `a=agent`, `d=dream`)
- `Task` interface only requires `Kill()` — spawn/render are caller-specific
- `StateBase` tracks output files, offsets, and notification state
- Terminal state guard prevents injecting messages into dead tasks

---

## 9. State Store & Session Management

**Pattern:** Thread-safe session persistence with JSON serialization.

**Location:** `internal/state/state.go`, `internal/state/session.go`

**Key Design Decisions:**
- `Session` struct tracks messages, model, turns, and metadata
- `SessionManager` handles save/load lifecycle
- Auto-save every 5 turns
- Compaction reduces token usage while preserving context
- Sessions stored in `~/.agent-harness/sessions/`

---

## 10. Secure Credential Storage

**Pattern:** AES-256-GCM encryption with Argon2id key derivation.

**Location:** `internal/config/secure.go`

**Key Design Decisions:**
- Master password required on startup
- Argon2id for secure key derivation (resistant to GPU attacks)
- AES-256-GCM for authenticated encryption
- File permissions 0600 (user read/write only)
- Atomic file writes prevent corruption
- Automatic migration from legacy plaintext configs

---

## 11. Layered Configuration

**Pattern:** Configuration layers with precedence: user → project → local.

**Location:** `internal/config/layered.go`

**Layers:**
1. **User**: `~/.agent-harness/settings.json`
2. **Project**: `./.agent-harness/settings.json`
3. **Local**: `./.agent-harness/settings.local.json` (gitignored)

**Features:**
- JSON-based with deep merge semantics
- Environment variable overrides
- MCP server configuration
- Permission mode defaults
- Always allow/deny lists

---

## 12. LLM Client Abstraction

**Pattern:** Provider-agnostic streaming client with OpenAI-compatible API mapping.

**Location:** `internal/llm/client.go`

**Key Design Decisions:**
- Supports OpenRouter, OpenAI, and Anthropic endpoints
- SSE parsing for streaming responses
- Tool calls mapped to OpenAI function-call format
- Cost tracking per model with usage estimation

---

## 13. Slash Command System

**Pattern:** Rich command system with history, completion, and formatted output.

**Location:** `internal/commands/slash.go`, `internal/ui/`

**Key Design Decisions:**
- Commands parsed from `/name args...` format
- Tab completion for command names
- History navigation (up/down arrows)
- Vim mode support (normal/insert/visual)
- Formatted output with lipgloss styles

**Available Commands:**
- `/help` — Show available commands
- `/status` — Session and workspace status
- `/clear` — Clear session history
- `/compact` — Compact session to reduce tokens
- `/cost` — Show token usage and estimated cost
- `/model` — Show/change current model
- `/permissions` — Show/change permission mode
- `/config` — Show configuration
- `/diff` — Show git diff
- `/export` — Export conversation to file
- `/session` — List/load saved sessions
- `/version` — Show version information
- `/quit`, `/exit` — Exit application

---

## 14. Git Integration

**Pattern:** Automatic git context detection for workspace awareness.

**Location:** `pkg/git/context.go`

**Key Design Decisions:**
- Auto-detects repository root, branch, and commit
- Shows uncommitted changes indicator
- `/diff` command displays workspace changes
- Remote URL detection for context

---

## 15. Cost Tracking

**Pattern:** Per-turn and cumulative cost estimation.

**Location:** `internal/agent/cost.go`

**Key Design Decisions:**
- Model-specific pricing (Claude, GPT-4, etc.)
- Token counting per turn
- Cumulative cost across session
- Formatted cost reports via `/cost` command

---

## 16. Message Utilities

**Pattern:** Normalization, sanitization, and boundary tracking for API-bound messages.

**Location:** `pkg/messages/messages.go`

**Key Design Decisions:**
- `NormalizeMessagesForAPI()` strips UI-only content
- `GetMessagesAfterCompactBoundary()` truncates to recent history
- Thinking blocks are never allowed as the final content block
- Signature blocks are stripped to preserve prompt cache

---

## 17. Plan Mode

**Pattern:** Explicit mode where the agent must outline its approach before acting.

**Location:** `internal/tools/builtin/plan.go`

**Key Design Decisions:**
- `EnterPlanModeTool` sets global `PlanState.Active`
- `ExitPlanModeTool` restores normal execution
- Plan mode is a permission context transformation (defer destructive tools)

---

## 18. Sandbox & Safety

**Pattern:** Path restrictions and dangerous command detection.

**Location:** `pkg/sandbox/sandbox.go`

**Key Design Decisions:**
- `IsPathAllowed()` checks working directory containment
- UNC paths rejected to prevent NTLM credential leaks
- `IsDangerousCommand()` blocks known harmful patterns

---

## 19. Bash Execution

**Pattern:** Shell command execution with timeout and context cancellation.

**Location:** `pkg/bash/bash.go`

**Key Design Decisions:**
- `context.WithTimeout` for deadline enforcement
- Combined output capture (stdout + stderr)
- Exit code extraction from `*exec.ExitError`

---

## 20. Docker Deployment (Optional)

**Pattern:** Multi-platform Docker images for containerized deployment.

**Location:** `Dockerfile`

### Building Docker Images Locally

```bash
# Build for current platform
docker build -t agent-harness:latest .

# Build for multiple platforms (requires buildx)
docker buildx create --use
docker buildx build --platform linux/amd64,linux/arm64 -t agent-harness:latest .
```

### Enabling Docker in CI/CD

The release workflow includes a commented-out Docker job. To enable it in your fork:

1. **Repository Settings** → **Actions** → **General**
2. Under "Workflow permissions", select **"Read and write permissions"**
3. Go to **Packages** → **Package settings**
4. Enable **"Inherit access from source repository"**
5. Uncomment the docker job in `.github/workflows/release.yml`

### Docker Job Configuration

```yaml
docker:
  needs: release
  runs-on: ubuntu-latest
  if: startsWith(github.ref, 'refs/tags/v')
  steps:
    - uses: actions/checkout@v4
    - uses: docker/setup-buildx-action@v3
    - uses: docker/login-action@v3
      with:
        registry: ghcr.io
        username: ${{ github.actor }}
        password: ${{ secrets.GITHUB_TOKEN }}
    
    - name: Get lowercase repo name
      id: repo
      run: |
        REPO=$(echo "${{ github.repository }}" | tr '[:upper:]' '[:lower:]')
        echo "IMAGE_NAME=$REPO" >> $GITHUB_OUTPUT
    
    - uses: docker/build-push-action@v5
      with:
        context: .
        platforms: linux/amd64,linux/arm64
        push: true
        tags: |
          ghcr.io/${{ steps.repo.outputs.IMAGE_NAME }}:latest
          ghcr.io/${{ steps.repo.outputs.IMAGE_NAME }}:${{ VERSION }}
```

### Running with Docker

```bash
# Run interactively
docker run -it --rm \
  -v $(pwd):/workspace \
  -e OPENROUTER_API_KEY=$OPENROUTER_API_KEY \
  ghcr.io/ba-calderonmorales/agent-harness:latest

# Run with persistent config
docker run -it --rm \
  -v $(pwd):/workspace \
  -v ~/.agent-harness:/home/agent/.agent-harness \
  ghcr.io/ba-calderonmorales/agent-harness:latest
```

---

## Directory Map

```
cmd/agent-harness/          # CLI entrypoint
internal/
  agent/                    # Core loop + streaming executor + cost tracking
  commands/                 # Slash command registry
  config/                   # Layered config + secure credential storage
  contextmgr/               # Compaction strategies
  llm/                      # LLM client abstraction
  permissions/              # Permission stack + classifier
  services/mcp/             # MCP manager stub
  state/                    # Session management + persistence
  tasks/                    # Task lifecycle registry
  tools/                    # Tool descriptor + registry
  tools/builtin/            # Built-in tool implementations
  ui/                       # Input handling + rendering
pkg/
  bash/                     # Shell execution
  git/                      # Git operations + context
  messages/                 # Message formatting
  sandbox/                  # Safety checks
  types/                    # Shared domain types
docs/                       # Documentation
scripts/                    # Install scripts
```

---

## Release Pipeline

**Pattern:** Multi-OS CD workflow with GitHub Releases.

**Location:** `.github/workflows/release.yml`

**Platforms:**
- Linux: amd64, arm64
- macOS: amd64 (Intel), arm64 (Apple Silicon)
- Windows: amd64, arm64

**Artifacts:**
- Tar.gz archives (Unix)
- Zip archives (Windows)
- SHA256 checksums
- Install scripts for curl-based installation
