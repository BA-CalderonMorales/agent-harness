# Loop architecture

The live agent loop is `internal/agent/loop.go`. App wiring constructs it in
`cmd/agent-harness/agent_turn.go`; provider and tool integrations stay behind
the existing runtime descriptors and callbacks.

## Control flow

A turn calls the LLM with the current context, consumes streamed text/tool
calls, executes accepted tools, adds results, and continues until completion,
cancellation, a configured limit or an unrecoverable error. Recovery and
continuation are bounded by the live configuration and state; consult the
contracts next to the code rather than assuming a fixed number of turns.

| Concern | Live location |
|---|---|
| Request loop and state | `internal/agent/loop.go` |
| Stream consumption | `internal/agent/loop_stream.go` |
| Failed-read recovery | `internal/agent/loop_recovery.go` |
| Repetition guard | `internal/agent/loop_repetition.go` |
| Context/token budget and compaction | `internal/agent/loop_compaction.go`, `prompt_tokens.go` |
| Streaming executor lifecycle | `internal/agent/executor.go`, `executor_tools.go` |
| Tool validation, permissions and execution | `internal/agent/executor_run.go` |
| Durable tool outcomes | `internal/agent/tool_result_receipt.go` |
| Tool descriptor/registry | `internal/runtime/tools/tool.go` |
| Built-in assembly and MCP tools | `cmd/agent-harness/tools_init.go` |
| Live approval policy | `cmd/agent-harness/tool_permissions.go`, `request_approval.go` |
| Session compaction/persistence | `internal/core/state/session.go`, `session_persistence.go` |

Concurrency-safe tools can run together; non-concurrent tools require exclusive
execution. Results retain their call identity and outcome. The executor's
lifecycle, ordering, cancellation and permission tests must keep passing when
its queue or progress behavior changes.

## Extending the loop

Add behavior where the live application constructs or calls it. A new built-in
uses the existing `tools.Tool` descriptor and assembly path, with validation,
permission classification, execution and result mapping tested together.
MCP adapters use the same tool pipeline; they are not a parallel approval path.

Shared interfaces such as `ToolBase` and the agent core types remain available
where already defined. Retaining an interface does not establish a runtime
implementation or justify adding a second executor with different semantics.
The Bucket suffix convention still applies to new domain implementations when
appropriate; it is not a requirement to rebuild retired factory scaffolding.

## Retired modular rewrite

The alternate `internal/session/loop/` and `internal/session/contextmgr/`
implementation was not wired into the application. The 0.4.1 cleanup removes
it together with its private bucket/default/factory graph. Old documentation
called this `internal/loop/`; that path is not a current entry point either.
The design is available in the
[v0.4.0 source](https://github.com/BA-CalderonMorales/agent-harness/tree/v0.4.0/internal/session).

The live `internal/agent/` loop, public helpers, parent tool/permission
interfaces and every existing regression test remain. Future orchestration,
background tasks and long-horizon capabilities need their own integration and
behavior evidence; the retired types were not proof of those capabilities.

## Verification

Run `go test -race ./...`, build/vet/static analysis, script contract checks,
and the real-terminal eval/security suite. Preserve tests for continuation,
compaction and resume, failed tool outcomes, approvals and cancellation.
Independent host checks establish objective eval outcomes; observation records
alone do not prove task completion. See [evals](../evals/README.md).
