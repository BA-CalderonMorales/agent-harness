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

**Pattern:** Layered, fail-closed permission evaluation.

**Location:** `internal/permissions/engine.go`

**Stack (top to bottom):**
1. Blanket deny rules
2. Blanket allow rules
3. Always-ask rules
4. Mode transformations (`dontAsk`, `bypassPermissions`, `auto`)
5. Tool-specific `CheckPermissions()`
6. Default: ask

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

**Location:** `internal/contextmgr/compact.go`

**Layers:**
1. **AutoCompact** — summarizes older messages when token threshold exceeded
2. **SnipCompact** — removes zombie messages and stale compact boundaries
3. **ContextCollapse** — advanced restructuring (extension point, no-op in base)

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

## 9. State Store

**Pattern:** Thread-safe generic state with snapshot and update semantics.

**Location:** `internal/state/state.go`

**Key Design Decisions:**
- `Store` is a `map[string]any` protected by `sync.RWMutex`
- `Update()` applies a transformation function
- `AppState` adds concrete fields: permission mode, model, file history
- `FileHistory` supports undo/rewind operations

---

## 10. Session Persistence

**Pattern:** Append-only JSONL log with durability tiers.

**Location:** `internal/state/persistence.go`

**Key Design Decisions:**
- User messages: blocking write + `fsync`
- Assistant messages: buffered write (fire-and-forget)
- Corrupted lines are skipped on read
- Output files are tracked per task for resume

---

## 11. LLM Client Abstraction

**Pattern:** Provider-agnostic streaming client with OpenAI-compatible API mapping.

**Location:** `internal/llm/client.go`

**Key Design Decisions:**
- Supports OpenRouter and Anthropic endpoints
- SSE parsing for streaming responses
- Tool calls mapped to OpenAI function-call format
- Anthropic `thinking` blocks passed through `extra_body`

---

## 12. MCP Integration Stub

**Pattern:** Pluggable Model Context Protocol server manager.

**Location:** `internal/services/mcp/mcp.go`

**Key Design Decisions:**
- Transport abstraction: `stdio`, `sse`, `http`, `ws`
- Connection lifecycle: `Connect -> List Tools -> CallTool -> Disconnect`
- Tool definitions dynamically registered from server schema
- Full implementation is an extension point

---

## 13. Slash Command Registry

**Pattern:** Commands are parsed from user input (`/name args...`) and dispatched via registry.

**Location:** `internal/commands/commands.go`

**Key Design Decisions:**
- Commands support aliases
- `Parse()` extracts name and args from raw input
- Handlers return structured results, not side effects directly
- Built-ins: `/clear`, `/compact`, `/model`, `/help`, `/exit`

---

## 14. Message Utilities

**Pattern:** Normalization, sanitization, and boundary tracking for API-bound messages.

**Location:** `pkg/messages/messages.go`

**Key Design Decisions:**
- `NormalizeMessagesForAPI()` strips UI-only content
- `GetMessagesAfterCompactBoundary()` truncates to recent history
- Thinking blocks are never allowed as the final content block
- Signature blocks are stripped to preserve prompt cache

---

## 15. Plan Mode

**Pattern:** Explicit mode where the agent must outline its approach before acting.

**Location:** `internal/tools/builtin/plan.go`

**Key Design Decisions:**
- `EnterPlanModeTool` sets global `PlanState.Active`
- `ExitPlanModeTool` restores normal execution
- Plan mode is a permission context transformation (defer destructive tools)

---

## 16. Sandbox & Safety

**Pattern:** Path restrictions and dangerous command detection.

**Location:** `pkg/sandbox/sandbox.go`

**Key Design Decisions:**
- `IsPathAllowed()` checks working directory containment
- UNC paths rejected to prevent NTLM credential leaks
- `IsDangerousCommand()` blocks known harmful patterns

---

## 17. Git Operations

**Pattern:** Abstracted git commands via `os/exec`.

**Location:** `pkg/git/git.go`

**Key Design Decisions:**
- `Repo` struct encapsulates path
- Operations use `git -C <path>` for safety
- Methods return errors, not panics

---

## 18. Bash Execution

**Pattern:** Shell command execution with timeout and context cancellation.

**Location:** `pkg/bash/bash.go`

**Key Design Decisions:**
- `context.WithTimeout` for deadline enforcement
- Combined output capture (stdout + stderr)
- Exit code extraction from `*exec.ExitError`

---

## Directory Map

```
cmd/agent-harness/          # CLI entrypoint
internal/
  agent/                    # Core loop + streaming executor
  commands/                 # Slash command registry
  config/                   # Environment/config loading
  contextmgr/               # Compaction strategies
  llm/                      # LLM client abstraction
  permissions/              # Permission stack + classifier
  services/mcp/             # MCP manager stub
  state/                    # In-memory store + persistence
  tasks/                    # Task lifecycle registry
  tools/                    # Tool descriptor + registry
  tools/builtin/            # Built-in tool implementations
pkg/
  bash/                     # Shell execution
  git/                      # Git operations
  messages/                 # Message formatting
  sandbox/                  # Safety checks
  types/                    # Shared domain types
```
