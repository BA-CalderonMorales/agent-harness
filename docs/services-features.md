# External Services & Feature Walls

> This document tracks capabilities that exist in Claude Code's architecture but are gated behind internal infrastructure, enterprise subscriptions, or feature flags. These are intentionally left as extension points in agent-harness.

---

## Tier 1: Internal-Only (Anthropic Employee / `USER_TYPE === 'ant'`)

These features are hard-gated in the source and cannot be recovered from published artifacts.

| Feature | Description | Extension Point |
|---------|-------------|-----------------|
| `REPLTool` | Interactive REPL with VM sandbox | `internal/tools/repl/` (optional build tag) |
| `ConfigTool` | Runtime configuration mutations | `internal/tools/config/` |
| `TungstenTool` | Performance monitoring panel | `internal/services/telemetry/` |
| `SuggestBackgroundPRTool` | Background PR suggestions | `internal/services/github/` |
| `VerifyPlanExecutionTool` | Plan verification agent | `internal/tools/verify/` |
| Agent nesting (unlimited) | Agents spawning agents recursively | `internal/agent/subagent.go` depth limiter |
| Dump system prompt | Prompt extraction for debugging | `internal/llm/debug.go` |

---

## Tier 2: Feature-Flag Gated (`feature('...')` from bundler)

These modules were dead-code-eliminated from the npm bundle. The TypeScript source references them but implementations are missing.

### Multi-Agent & Coordination

| Feature | Flag | Pattern |
|---------|------|---------|
| `COORDINATOR_MODE` | Multi-agent coordinator with worker agents | `internal/agent/coordinator.go` |
| `FORK_SUBAGENT` | Fork-process subagents | `internal/agent/fork.go` |
| `UDS_INBOX` | Peer discovery via Unix domain sockets | `internal/agent/peers.go` |
| `AGENT_TRIGGERS` | Cron-based agent triggers | `internal/tasks/cron.go` |
| `AGENT_TRIGGERS_REMOTE` | Remote webhooks triggering agents | `internal/tasks/remote_trigger.go` |

### Context & Memory

| Feature | Flag | Pattern |
|---------|------|---------|
| `CONTEXT_COLLAPSE` | Advanced context restructuring | `internal/services/compact/collapse.go` |
| `HISTORY_SNIP` | Aggressive history trimming | `internal/services/compact/snip.go` |
| `CACHED_MICROCOMPACT` | Cache-editing micro-compaction | `internal/services/compact/microcompact.go` |
| `REACTIVE_COMPACT` | Preemptive compaction on 413 errors | `internal/services/compact/reactive.go` |
| `EXPERIMENTAL_SKILL_SEARCH` | Remote skill discovery | `internal/skills/search.go` |

### Productivity & Workflow

| Feature | Flag | Pattern |
|---------|------|---------|
| `WORKFLOW_SCRIPTS` | Workflow execution engine | `internal/tasks/workflow.go` |
| `WEB_BROWSER_TOOL` | Browser automation | `internal/tools/browser/` |
| `TERMINAL_PANEL` | Terminal capture panel | `internal/tools/terminal/` |
| `MONITOR_TOOL` | MCP monitoring | `internal/services/mcp/monitor.go` |

### Proactive / KAIROS

| Feature | Flag | Pattern |
|---------|------|---------|
| `KAIROS` | Fully autonomous assistant mode | `internal/agent/kairos.go` |
| `KAIROS_PUSH_NOTIFICATION` | Push notifications | `internal/services/notifications/` |
| `KAIROS_GITHUB_WEBHOOKS` | GitHub PR subscriptions | `internal/services/github/webhooks.go` |
| `PROACTIVE` | Sleep tool, proactive behavior | `internal/tools/sleep.go` |
| `DAEMON` | Background daemon workers | `cmd/agent-harness-daemon/` |

### Misc

| Feature | Flag | Pattern |
|---------|------|---------|
| `VOICE_MODE` | Voice input/output | `internal/ui/voice.go` |
| `OVERFLOW_TEST_TOOL` | Testing utility | `internal/tools/testing/` |
| `BUDDY` | Companion sprite notifications | `internal/ui/buddy.go` |
| `TORCH` | Internal debug tool | `internal/tools/torch.go` |

---

## Tier 3: External Service Dependencies

These require third-party accounts, APIs, or infrastructure.

| Service | What It Does | Config Key | Notes |
|---------|--------------|------------|-------|
| Anthropic API | Primary LLM backend | `anthropic.api_key` | Default; requires API key or OAuth |
| OpenRouter | Aggregated model access | `openrouter.api_key` | Supported as first-class alternative |
| GrowthBook | Feature flags & A/B tests | `growthbook.api_key` | Runtime feature gating |
| Statsig / Datadog | Telemetry sinks | `telemetry.enabled` | 1P analytics + 3P observability |
| GitHub API | PRs, issues, repo operations | `github.token` | Needed for PR tools |
| MCP Servers | External tool servers | `mcp.servers` | Stdio/SSE/WS/HTTP transports |
| Claude Desktop Bridge | Remote session relay | `bridge.enabled` | Enterprise remote dev environments |
| Remote Managed Settings | Policy enforcement | `remote_settings.url` | `/api/claude_code/settings` equivalent |

---

## Tier 4: Enterprise / Subscription Gated

These are visible in the source but require specific subscription tiers.

| Capability | Gating | Our Stance |
|------------|--------|------------|
| Remote managed settings (accept-or-die dialog) | Enterprise / Team OAuth | Not implemented; local config only |
| Fast mode (`penguin_mode`) | API entitlement | Exposed as model-selection preference |
| Bypass permissions mode | Killswitch-gated | Not implemented; always require approval |
| Auto mode classifier | Available to all in original | Implemented as optional safety layer |
| OAuth flows (console, GitHub, Slack) | Console / Pro | API-key only for now |

---

## How to Enable a Feature

Each tier-2 feature has a corresponding **stub interface** or **build tag**:

```go
// +build kairos

package agent

// KairosMode enables autonomous tick-based agent execution.
// Implement this interface and register it in the agent harness.
type KairosMode interface {
    Tick(ctx context.Context) error
}
```

To add a feature:
1. Create the package under the extension point listed above
2. Add a build tag or config gate
3. Register it in the appropriate factory (`ToolRegistry`, `TaskRegistry`, or `Agent`)
4. Document it here

---

## Design Principle

> We are a one-person show. The foundation must be solid before the skyscraper goes up.

The core loop (agent -> tools -> permissions -> LLM) is fully functional without any tier-2 or tier-3 features. Everything else is an **extension surface**, not a hard dependency.
