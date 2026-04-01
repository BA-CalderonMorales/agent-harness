# agent-harness

A clean-room, pattern-derived agent harness for building coding agents like Claude Code, OpenCode, Gemini CLI, and similar tools.

> This project captures the architectural patterns from production agentic coding tools and reimplements them in Go, with first-class support for OpenRouter and other OpenAI-compatible endpoints.

---

## Purpose

`agent-harness` exists to teach the community how to build professional-grade coding agents. It is not a copy of any existing product. Instead, it derives the **domain patterns** that make agentic coding work:

1. The core agent loop
2. Tool dispatch and execution
3. Permission and safety layers
4. Context compaction and memory
5. Sub-agent orchestration
6. MCP integration
7. Streaming UI

---

## Quick Start

```bash
# Set your API key (OpenRouter or Anthropic)
export OPENROUTER_API_KEY="sk-or-v1-..."
# or
export ANTHROPIC_API_KEY="sk-ant-..."

# Run the agent
go run ./cmd/agent-harness
```

---

## Architecture

```
cmd/agent-harness/          # CLI entrypoint
internal/
  agent/                    # Core query loop, message handling
  tools/                    # Tool interface, registry, built-in tools
  permissions/              # Permission rules, auto-mode classifier
  llm/                      # LLM client abstraction (OpenRouter, Anthropic)
  contextmgr/               # Context compaction, token budgeting
  tasks/                    # Background/local/remote task system
  commands/                 # Slash command dispatch
  state/                    # In-memory state store
  ui/                       # Terminal UI (bubbletea)
  services/
    mcp/                    # MCP client management
    compact/                # Context compression
pkg/
  git/                      # Git operations
  bash/                     # Shell execution
  sandbox/                  # Path and command sandboxing
  fs/                       # File utilities
  messages/                 # Message formatting
```

---

## Derived Patterns

See the source code for pattern implementations derived from domain analysis:

- `internal/tools/tool.go` — The descriptor-object tool pattern
- `internal/agent/loop.go` — The streaming agent loop with recovery
- `internal/permissions/engine.go` — Layered permission decision stack
- `internal/contextmgr/compact.go` — Three-layer context compression
- `internal/tasks/registry.go` — Task lifecycle and backgrounding

---

## Feature Walls & Extension Points

For capabilities that require internal infrastructure, enterprise subscriptions, or third-party services, see:

- [`services-features.md`](services-features.md) — What is gated and where to plug it in
- [`edgecases.md`](edgecases.md) — Non-obvious behaviors you must design for

---

## License

MIT
