# agent-harness

A clean-room, pattern-derived agent harness for building coding agents.

> This project captures architectural patterns from production agentic coding tools and reimplements them in Go, with first-class support for OpenRouter and other OpenAI-compatible endpoints.

## Purpose

`agent-harness` exists to teach the community how to build professional-grade coding agents. It derives the **domain patterns** that make agentic coding work:

1. Core agent loop with streaming responses
2. Tool dispatch and execution with permission controls
3. Permission modes (read-only / workspace-write / danger-full-access)
4. Secure credential storage with AES-256-GCM encryption
5. Session management with auto-save and compaction
6. Layered configuration (user / project / local)
7. Slash command system with history and completion
8. Git integration for workspace context
9. Cost tracking with model-specific pricing
10. MCP integration (extension point)

## Quick Start

### Installation

**Linux/macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/BA-CalderonMorales/agent-harness/main/scripts/install.sh | bash
```

**Termux (Android):**
```bash
curl -fsSL https://raw.githubusercontent.com/BA-CalderonMorales/agent-harness/main/scripts/install-termux.sh | bash
```

**Manual:**
```bash
go install github.com/BA-CalderonMorales/agent-harness/cmd/agent-harness@latest
```

### First Run

On first run, you will be prompted to:
1. Choose an API provider (OpenRouter, OpenAI, or Anthropic)
2. Enter your API key (input is masked)
3. Select a model
4. Set a master password for credential encryption

### Usage

```bash
# Start interactive mode
agent-harness

# Or use the short alias (after setup)
ah
```

**Slash Commands:**
- `/help` — Show available commands
- `/status` — Show session and workspace status
- `/cost` — Show token usage and estimated cost
- `/compact` — Compact session to reduce token usage
- `/model <name>` — Change the current model
- `/permissions <mode>` — Change permission mode
- `/diff` — Show git diff of workspace changes
- `/export` — Export conversation to file
- `/quit` — Exit the application

## Features

### Secure Credential Storage
- AES-256-GCM encryption for API keys
- Argon2id key derivation
- Master password required on startup
- File permissions 0600 (user-only access)

### Layered Configuration
1. **User**: `~/.agent-harness/settings.json`
2. **Project**: `./.agent-harness/settings.json`
3. **Local**: `./.agent-harness/settings.local.json` (gitignored)

### Permission Modes
- **read-only**: Only read/search tools allowed
- **workspace-write**: Most tools allowed, dangerous ones ask
- **danger-full-access**: All tools run without confirmation

## Architecture

```
cmd/agent-harness/          # CLI entrypoint
internal/
  agent/                    # Core loop + streaming executor
  commands/                 # Slash command registry
  config/                   # Layered config + secure storage
  llm/                      # LLM client abstraction
  permissions/              # Permission stack
  state/                    # Session management
  tools/                    # Tool descriptor + registry
  tools/builtin/            # Built-in tool implementations
  ui/                       # Input handling + rendering
pkg/
  bash/                     # Shell execution
  git/                      # Git operations
  sandbox/                  # Safety checks
```

See [docs/architecture.md](docs/architecture.md) for detailed documentation.

## Documentation

- [Architecture](docs/architecture.md) — Pattern implementations
- [Edge Cases](docs/edgecases.md) — Non-obvious behaviors

## Building from Source

```bash
go build -o agent-harness ./cmd/agent-harness
```

## License

MIT

## Acknowledgments

This project is inspired by the architectural patterns found in [terminal-jarvis](https://github.com/BA-CalderonMorales/terminal-jarvis).
