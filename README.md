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
6. Root YAML plus layered configuration (user / project / local)
7. Slash command system

## Quick Start

### Local Model First

The repository includes `agent-harness.yml`, a local-first runtime config for
an OpenAI-compatible `llama.cpp` server. The intended default model is
DeepReinforce Ornith-1.0 GGUF, using a practical `Q4_K_M` quantization.

```bash
git clone https://github.com/BA-CalderonMorales/agent-harness.git
cd agent-harness
mkdir -p models

# Place an Ornith-1.0 Q4_K_M GGUF at:
# ./models/ornith-1.0-9b-Q4_K_M.gguf

llama-server -m ./models/ornith-1.0-9b-Q4_K_M.gguf -c 8192 --host 127.0.0.1 --port 8080
```

In another terminal:

```bash
make build
./build/agent-harness
```

Run `./build/agent-harness --diagnose` to inspect resolved config, model file
presence, and local endpoint reachability. See
[Local Model Setup](docs/local-models.md) for download and override details.

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

## Configuration

`agent-harness.yml` is the obvious project setup file. Environment variables
override YAML values when needed:

- `AH_PROVIDER`, `AH_RUNTIME`, `AH_MODEL`, `AH_MODEL_PATH`
- `AH_ENDPOINT_URL`, `AH_CONTEXT_LENGTH`, `AH_TEMPERATURE`, `AH_MAX_TOKENS`
- `AH_WORKSPACE_PATH`, `AH_LOCAL_SERVER_COMMAND`
- `AH_PERMISSION_MODE`, `AH_PERM_READ`, `AH_PERM_WRITE`, `AH_PERM_DELETE`, `AH_PERM_EXECUTE`

Remote providers are still supported through OpenRouter, OpenAI, and Anthropic.
Set `AH_PROVIDER`, `AH_MODEL`, and `AH_API_KEY` or use the `/login` flow.

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
- [Local Model Setup](docs/local-models.md) - Ornith-1.0 GGUF and local runtime setup

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
