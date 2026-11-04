# Agent Harness Architecture

The terminal application is wired in `cmd/agent-harness/`. Its live agent loop
is `internal/agent/`; tools, providers, state and the interface have separate
domain boundaries. This page describes the running application, not a catalog
of features from another coding agent.

## A turn through the application

1. App wiring resolves configuration, credentials, model and session state,
   registers built-in tools and configured MCP tools, and starts the TUI.
2. A submitted prompt reaches the live `agent.Loop`. The request includes the
   system prompt, enabled tool descriptors and conversation history.
3. The LLM client streams text and tool calls. The executor validates tool
   input, evaluates permission decisions and requests approval when required.
4. Tool results carry the tool-call ID and error outcome back into the loop.
   The loop may continue, recover within its limits, or finish the turn.
5. TUI events display progress and results; application wiring persists session
   and compaction state. Cancellation and continuation contracts are covered
   beside the implementation.

See [loop architecture](loop-architecture.md) and
[conversation flow](conversation_flow.md) for the lifecycle details.

## Domain map

| Domain | Responsibility |
|---|---|
| `cmd/agent-harness/` | Boot, command wiring, provider/settings changes, approval callbacks, session/TUI integration. |
| `internal/agent/` | Live request/continuation loop, streaming executor, recovery, compaction, token and cost accounting, tool-result receipts. |
| `internal/agents/`, `internal/skills/` | Agent definitions and dynamically discovered skill content. |
| `internal/core/config/` | Layered settings, environment overrides, credential store and configuration/data paths. |
| `internal/core/state/` | Sessions, persistence, restoration, compaction and export. |
| `internal/core/audit/`, `diag/`, `persona/` | Tool audit records, diagnostics and behavior/persona context. |
| `internal/interface/tui/` | Bubble Tea views and interaction state; approval/question overlays and opt-in observation streams. |
| `internal/interface/commands/`, `approval/` | Slash commands and shared approval types. |
| `internal/runtime/llm/` | Provider requests, discovery, streaming protocol parsing and bounded error/retry handling. |
| `internal/runtime/tools/` | Tool descriptor/registry interfaces, built-ins, filesystem cache and MCP adapters. |
| `internal/runtime/services/mcp/` | Configured MCP subprocess connections, protocol, lifecycle and result conversion. |
| `internal/runtime/permissions/` | Permission engine and classifier contracts, also exercised by e2e tests; the app's live approval wiring is in `cmd/agent-harness/tool_permissions.go`. |
| `internal/ui/` | Retained terminal input and rendering support still imported by app wiring. |
| `internal/testharness/`, `e2e/` | Isolated homes, MCP peer, real-model terminal frames and behavior contracts. |
| `pkg/` | Shared messages, types, formatting and git helpers. Public bash/sandbox helpers remain for compatibility; they are not the live shell executor. |
| `evals/` | Real-terminal journeys, generated task campaigns, independent host verification and bounded resource/evidence collection. |

`internal/core/planning/` is a tested workflow utility outside the app dependency
chain. It remains deliberately; its existence does not mean the live agent
runs an autonomous self-improvement planner.

## Permissions and process boundaries

The app combines agent mode, configured permission settings, tool-specific
checks, approval callbacks and audit records. Manual mode asks, auto mode
skips interactive prompts within the permission machinery, plan mode gates
mutations, and chat mode sends no tools. See [command approval](command-approval.md)
and the tests next to `tool_permissions.go` for precedence and exceptions.

Tool descriptors live in `internal/runtime/tools/tool.go`; built-ins are
registered in `cmd/agent-harness/tools_init.go`. The live shell executor is
`internal/runtime/tools/builtin/bash_exec.go`, including platform-specific
process helpers. Do not substitute the public `pkg/bash` helper or infer
containment from the `pkg/sandbox` name.

The harness normally runs with the user's privileges. Project configuration
and MCP commands are executable content; command warnings are not a kernel
boundary. [Security review](security-review.md) records accepted risks, and
[OpenShell](openshell.md) describes optional external containment.

## State, credentials and context

Configuration and data roots are resolved by `internal/core/config/paths.go`.
Credentials use the encrypted store described in [credentials](credentials.md);
the local machine key is not a master-password prompt. Session paths and export
behavior are implemented in `internal/core/state/`.

The live loop's compaction logic is `internal/agent/loop_compaction.go`.
Session compaction is in `internal/core/state/session.go`; application wiring
synchronizes the durable result. Do not replace these with a historical
context-manager implementation. Existing compaction and resume tests are part
of the required regression surface.

## Retired scaffolding and future work

The 0.4.1 cleanup retires the unwired `internal/session/` loop/context manager,
alternate executor and filesystem/permission buckets, and unused environment,
service and task scaffolding. Their source remains in the
[v0.4.0 tree](https://github.com/BA-CalderonMorales/agent-harness/tree/v0.4.0/internal).
No active implementation or regression test was removed with them.

Add a future capability through a tested live entry point. A registry, stub,
or proposed extension in a historical document does not establish that the
application supports it. [Services and features](services-features.md) separates
current integration boundaries from future ideas.

## Build and release

`make verify` runs structure guidance and the Go suite. The CI workflows also
cover quality, security, Linux terminal evals, Windows and container builds.
The release workflow builds six platform archives and checksums from a tag
whose version must match `cmd/agent-harness/main.go`.

Releases use the two-PR flow in
[the release runbook](../.agent-harness/skills/release-workflow/SKILL.md).
Generated evidence and local campaign results are excluded from source
checkouts/build context under the [retention policy](../evidence/README.md).
