# agent-harness

A clean-room, pattern-derived agent harness for building coding agents like Claude Code, OpenCode, Gemini CLI, and similar tools.

> This project captures the architectural patterns from production agentic coding tools and reimplements them in Go, with first-class support for OpenRouter and other OpenAI-compatible endpoints.

---

## Purpose

`agent-harness` exists to teach the community how to build professional-grade coding agents. It is not a copy of any existing product. Instead, it derives the **domain patterns** that make agentic coding work:

1. The core agent loop with streaming responses
2. Tool dispatch and execution with permission controls
3. Permission modes (read-only / workspace-write / danger-full-access)
4. Secure credential storage with AES-256-GCM encryption
5. Session management with auto-save and compaction
6. Layered configuration (user / project / local)
7. Slash command system with history and completion
8. Git integration for workspace context
9. Cost tracking with model-specific pricing
10. MCP integration (extension point)

---

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

**From Release:**
Download the appropriate binary for your platform from the [releases page](https://github.com/BA-CalderonMorales/agent-harness/releases).

### First Run

On first run, you'll be prompted to:
1. Choose an API provider (OpenRouter, OpenAI, or Anthropic)
2. Enter your API key (input is masked)
3. Select a model
4. Set a master password for credential encryption

Your credentials are encrypted with AES-256-GCM and stored with 0600 permissions.

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

---

## Features

### 🔐 Secure Credential Storage
- AES-256-GCM encryption for API keys
- Argon2id key derivation (resistant to GPU attacks)
- Master password required on startup
- Password masking during input
- File permissions 0600 (user-only access)
- Automatic migration from legacy plaintext configs

### ⚙️ Layered Configuration
Configuration layers with precedence (later overrides earlier):
1. **User**: `~/.agent-harness/settings.json`
2. **Project**: `./.agent-harness/settings.json`
3. **Local**: `./.agent-harness/settings.local.json` (gitignored)

Supports permission modes, always allow/deny lists, MCP servers, and custom environment variables.

### 🛡️ Permission Modes
Three permission modes for different safety levels:
- **read-only**: Only read/search tools allowed
- **workspace-write**: Most tools allowed, dangerous ones ask
- **danger-full-access**: All tools run without confirmation

### 💬 Session Management
- Persistent JSON session storage
- Auto-save every 5 turns
- Session compaction to reduce token usage
- Load and resume previous sessions
- Token estimation for context window management

### ⌨️ Rich Terminal UI
- Command history (up/down navigation)
- Tab completion for slash commands
- Vim mode support (normal/insert)
- Formatted output with color-coded sections
- Progress indicators for long operations

### 📊 Cost Tracking
- Per-turn token counting
- Cumulative usage across session
- Model-specific pricing (Claude, GPT-4, etc.)
- Detailed cost reports via `/cost` command

### 🔧 Git Integration
- Automatic project root detection
- Branch display in status
- Git diff via `/diff` command
- Uncommitted changes indicator

---

## Architecture

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
```

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for detailed pattern documentation.

---

## Supported Platforms

| Platform | AMD64 | ARM64 |
|----------|-------|-------|
| Linux | ✅ | ✅ |
| macOS | ✅ | ✅ |
| Windows | ✅ | ✅ |

---

## Documentation

- [Architecture](docs/ARCHITECTURE.md) — Detailed pattern implementations
- [Feature Parity](docs/PARITY.md) — Comparison with claw-code reference
- [Edge Cases](docs/edgecases.md) — Non-obvious behaviors to design for
- [Services & Features](docs/services-features.md) — Extension points and gated features

---

## Building from Source

**Requirements:**
- Go 1.26.1 or later

**Build:**
```bash
go build -o agent-harness ./cmd/agent-harness
```

**Install locally:**
```bash
go install ./cmd/agent-harness
```

---

## Development

**Run tests:**
```bash
go test ./...
```

**Run with verbose output:**
```bash
agent-harness --verbose
```

**Enable TUI mode:**
```bash
agent-harness --tui
```

---

## Contributing

This project follows clean-room implementation principles:
1. Domain patterns are derived from observable behavior
2. No proprietary code is copied
3. All implementations are original
4. Documentation explains the "why" behind patterns

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) to understand the patterns before contributing.

---

## License

MIT

---

## Acknowledgments

This project is inspired by the architectural patterns found in:
- Claude Code
- OpenCode
- Gemini CLI
- Continue.dev

We thank these projects for demonstrating what professional-grade agentic coding tools can be.
