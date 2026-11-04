# Services and extension boundaries

This page describes integrations present in agent-harness. Historical
comparisons with features from other coding agents are available in the
[v0.4.0 document](https://github.com/BA-CalderonMorales/agent-harness/blob/v0.4.0/docs/services-features.md);
those comparisons are not declarations that an interface, service or build tag
exists in the current app.

## Current integrations

| Integration | Code and behavior |
|---|---|
| LLM providers | `internal/runtime/llm/`: configured endpoints, model discovery, streaming responses and provider-specific request handling. See [model configuration](supported_models.md) and [Lemonade](lemonade.md). |
| MCP servers | `internal/runtime/services/mcp/`: configured subprocess clients, connection lifecycle and result conversion. `internal/runtime/tools/mcp/` adapts them into the tool pipeline. See [MCP evidence](mcp-evidence.md). |
| Git context and commands | `pkg/git/` and app/command wiring; shell execution still follows tool approvals. |
| Credentials | `internal/core/config/`: encrypted local store and explicit secret references. See [credentials](credentials.md) and its trust limits. |
| Sessions and audit | `internal/core/state/`, `audit/` and `diag/`; local persistence, tool outcomes, export and diagnostics. |
| Named agents and skills | `internal/agents/`, `internal/skills/`, and the registered agent tool. See [sub-agents](subagents.md) for actual definition/loading behavior. |
| Evaluation campaigns | `evals/`: bounded real-terminal comparisons, retained failures and independent host checks. They screen changes and do not deploy them automatically. |
| External containment | Optional [OpenShell](openshell.md) configuration and examples; not an in-process security boundary. |

## Future capabilities

Background task lifecycles, remote orchestration, stronger workspace trust and
autonomous improvement need concrete integration work, resource budgets and
regression evidence. There is no dormant task registry or modular session
factory to switch on: the unwired versions were retired in the 0.4.1 cleanup.
The tested `internal/core/planning/` utility remains outside the live app path.

Start future work from the [architecture](architecture.md) and
[live loop](loop-architecture.md), identify the real entry point, and add a
behavior contract before claiming the feature works. Do not infer support
from an old feature-flag name or a permissive build with an unused tag.
