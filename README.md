# agent-harness

A clean-room, pattern-derived agent harness for building coding agents.

> **Note:** This project is in early development. We are iterating fast. Best used in Coder, DevPod, or GitHub Codespaces for a consistent environment.

## Purpose

`agent-harness` captures architectural patterns from production agentic coding tools:

1. Core agent loop with streaming responses
2. Tool dispatch with permission controls
3. Two execution modes: interactive (prompt for each command) and yolo (auto-approve with visibility)
4. Secure credential storage with AES-256-GCM encryption
5. Session management with auto-save
6. Layered configuration (user / project / local)
7. Slash command system

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

### Usage

```bash
# Start the TUI
agent-harness

# Or use the short alias (after setup)
ah
```

**Key Controls:**
- `Tab` / `Shift+Tab` - Switch views (Chat, Sessions, Settings)
- `ESC` - Cancel current agent execution or exit mode
- `?` - Show help (in normal mode)
- `/` - Open command palette (when input is empty)
- `Ctrl+C` - Quit

**Execution Modes:**
- **Interactive** (default) - Prompts you before executing shell/write/edit commands
- **Yolo** - Auto-approves commands but shows what is happening in the UI

Switch modes in Settings or with `/mode` commands.

## Architecture

```
cmd/agent-harness/          # CLI entrypoint
internal/
  agent/                    # Core loop + streaming executor
  approval/                 # Command approval system
  commands/                 # Slash command registry
  config/                   # Layered config + secure storage
  llm/                      # LLM client abstraction
  permissions/              # Permission stack
  state/                    # Session management
  tools/                    # Tool descriptor + registry
  tui/                      # Terminal UI (Bubble Tea)
pkg/
  bash/                     # Shell execution
  git/                      # Git operations
```

## Documentation

- [Architecture](docs/architecture.md) - Pattern implementations
- [Command Approval](docs/command-approval.md) - How the approval system works
- [Edge Cases](docs/edgecases.md) - Non-obvious behaviors

## Building from Source

```bash
go build -o agent-harness ./cmd/agent-harness
```

## License

MIT

## Acknowledgments

This project is inspired by the architectural patterns found in [terminal-jarvis](https://github.com/BA-CalderonMorales/terminal-jarvis).

The TUI design patterns are inspired by [golazo](https://github.com/0xjuanma/golazo) by [Juan Manuel](https://github.com/0xjuanma).

Additional TUI inspiration from the [awesome-tuis](https://github.com/rothgar/awesome-tuis) collection.
