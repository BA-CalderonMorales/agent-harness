# Usage Guide

Complete guide to using agent-harness effectively.

---

## Table of Contents

- [Getting Started](#getting-started)
- [Basic Usage](#basic-usage)
- [Slash Commands](#slash-commands)
- [Permission Modes](#permission-modes)
- [Configuration](#configuration)
- [Session Management](#session-management)
- [Cost Tracking](#cost-tracking)
- [Git Integration](#git-integration)
- [Tips and Tricks](#tips-and-tricks)

---

## Getting Started

### First Run

When you start agent-harness for the first time:

```bash
agent-harness
```

You'll see:

```
╔════════════════════════════════════════════════════════════╗
║     Agent Harness - Secure Initial Setup                   ║
╚════════════════════════════════════════════════════════════╝

No API credentials found. Let's set them up securely.

Choose an API provider:
  1) OpenRouter (recommended - access to multiple models)
  2) OpenAI
  3) Anthropic

Enter choice (1-3) [1]: 
```

Follow the prompts to:
1. Select your API provider
2. Enter your API key (input is masked for security)
3. Choose a model
4. Set a master password for credential encryption

### Starting agent-harness

```bash
# Start interactive mode
agent-harness

# Start with specific model
agent-harness --model claude-3-5-sonnet-20241022

# Start with read-only permissions
agent-harness --permission-mode read-only
```

---

## Basic Usage

Once in the REPL, you can:

1. **Type messages** to chat with the AI
2. **Use slash commands** for special functions (starting with `/`)
3. **Press Ctrl+C** to cancel the current operation
4. **Type `/quit` or Ctrl+D** to exit

### Example Session

```
> /status

Status
  Session mode     active
  Messages         5
  Turns            2
  Est. tokens      1240
  Model            claude-3-5-sonnet-20241022

Workspace
  Project root     /home/user/myproject
  Git branch       feature/new-feature

Next
  /status     Show this status report
  /compact    Trim session if getting large
  /cost       Show token usage and cost

> How do I implement a REST API in Go?

I'll help you implement a REST API in Go. Let me start by checking your current project structure...

[Using tool: glob]

Based on your project, here's how to implement a REST API in Go:
...

> /cost

Cost
  Input tokens     450
  Output tokens    890
  Total tokens     1340

Next
  /status     See session + workspace context
  /compact    Trim local history if the session is getting large

> /quit

Goodbye.
Cost: 450 input + 890 output tokens (~$0.0032) across 3 turns
```

---

## Slash Commands

### Core Commands

| Command | Description | Example |
|---------|-------------|---------|
| `/help` | Show available commands | `/help` |
| `/status` | Show session and workspace status | `/status` |
| `/quit` | Exit the application | `/quit` |
| `/exit` | Alias for `/quit` | `/exit` |

### Session Commands

| Command | Description | Example |
|---------|-------------|---------|
| `/clear` | Clear session history (requires `--confirm`) | `/clear --confirm` |
| `/compact` | Compact session to reduce token usage | `/compact` |
| `/session` | List or load saved sessions | `/session`, `/session load abc123` |
| `/export` | Export conversation to file | `/export`, `/export my-session.json` |

### Settings Commands

| Command | Description | Example |
|---------|-------------|---------|
| `/model` | Show or change the current model | `/model`, `/model gpt-4o` |
| `/permissions` | Show or change permission mode | `/permissions`, `/permissions read-only` |
| `/config` | Show configuration | `/config` |
| `/memory` | Show memory context | `/memory` |

### Information Commands

| Command | Description | Example |
|---------|-------------|---------|
| `/cost` | Show token usage and cost | `/cost` |
| `/diff` | Show git diff of workspace changes | `/diff` |
| `/version` | Show version information | `/version` |

### Advanced Commands

| Command | Description | Example |
|---------|-------------|---------|
| `/agents` | List/manage sub-agents | `/agents` |
| `/skills` | List/manage skills | `/skills` |

---

## Permission Modes

agent-harness has three permission modes for different safety levels:

### Read-Only Mode

```
> /permissions read-only

Permissions updated
  Previous mode    workspace-write
  Active mode      read-only
  Applies to       Subsequent tool calls in this session
```

In read-only mode:
- ✅ Read, search, and analyze tools work
- ❌ Bash, write, and edit tools are blocked
- Use for: Code review, analysis, safe exploration

### Workspace-Write Mode (Default)

```
> /permissions workspace-write
```

In workspace-write mode:
- ✅ Most tools work automatically
- ⚠️ Dangerous tools (bash, destructive operations) ask for confirmation
- Use for: Active development, refactoring, most coding tasks

### Danger-Full-Access Mode

```
> /permissions danger-full-access

Warning: This allows all tools to run without confirmation!
```

In danger-full-access mode:
- ✅ All tools run without confirmation
- ⚠️ Be careful - the AI can make any changes
- Use for: Trusted workflows, CI/CD, when you know what you're doing

---

## Configuration

### Layered Configuration

agent-harness uses three configuration layers:

1. **User config** (`~/.agent-harness/settings.json`)
   - Applies to all projects
   - Set your default model here

2. **Project config** (`./.agent-harness/settings.json`)
   - Committed to version control
   - Project-specific settings

3. **Local config** (`./.agent-harness/settings.local.json`)
   - Gitignored
   - Personal overrides for the project

### Example Configuration

```json
{
  "provider": "openrouter",
  "model": "anthropic/claude-3.5-sonnet",
  "permission_mode": "workspace-write",
  "always_allow": ["read", "glob", "grep"],
  "always_deny": ["bash"],
  "mcpServers": {
    "filesystem": {
      "transport": "stdio",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/allowed"]
    }
  }
}
```

### Environment Variables

You can also use environment variables:

```bash
export OPENROUTER_API_KEY="sk-or-v1-..."
export ANTHROPIC_API_KEY="sk-ant-..."
export OPENAI_API_KEY="sk-..."
export AGENT_HARNESS_MODEL="claude-3-5-sonnet-20241022"
export AGENT_HARNESS_PERMISSION_MODE="read-only"
```

---

## Session Management

### Understanding Sessions

A session is a conversation with the AI that persists across restarts:
- Messages are saved automatically every 5 turns
- Sessions are stored in `~/.agent-harness/sessions/`
- Each session has a unique ID

### Listing Sessions

```
> /session

Saved sessions:
  abc123def - 15 messages, 3 turns (2026-04-01)
  xyz789abc - 42 messages, 8 turns (2026-03-31)
```

### Loading a Session

```
> /session load abc123def

Session loaded: abc123def
```

### Compacting Sessions

When sessions get too large, compact them:

```
> /compact

Compact
  Result           compacted
  Messages removed 35
  Messages kept    7
  Tip              Use /status to review the trimmed session
```

Compaction removes older messages while preserving recent context.

### Exporting Sessions

Save a session to a file:

```
> /export

Export
  Result           wrote transcript
  File             session-abc123.txt
```

Exports are redacted support logs by default. API keys, tokens, secrets, and
local absolute paths are obfuscated before writing the file. Use
`/export --format markdown report.md` or `/export --format json report.json`
when maintainers need another format.

---

## Cost Tracking

### Viewing Costs

```
> /cost

Cost
  Input tokens     1240
  Output tokens    2890
  Cache create     0
  Cache read       0
  Total tokens     4130

Next
  /status     See session + workspace context
  /compact    Trim local history if the session is getting large
```

### Understanding Costs

Costs are estimated based on:
- **Input tokens**: Your messages + context
- **Output tokens**: AI responses
- **Model pricing**: Different models have different rates

Actual costs may vary slightly from estimates.

### Reducing Costs

1. **Use cheaper models** for simple tasks:
   ```
   > /model claude-3-5-haiku
   ```

2. **Compact sessions** regularly:
   ```
   > /compact
   ```

3. **Clear sessions** when starting new tasks:
   ```
   > /clear --confirm
   ```

---

## Git Integration

### Automatic Context

agent-harness automatically detects:
- Git repository root
- Current branch
- Uncommitted changes

### Git Diff

See uncommitted changes:

```
> /diff

Changes:
 README.md          | 5 +++++
 main.go            | 10 ++++++++++
 2 files changed, 15 insertions(+)
```

### Working with Git

You can ask the AI to help with git operations:

```
> Can you show me a summary of recent commits?

[Using tool: bash]

Recent commits:
  a1b2c3d feat: add user authentication
  e4f5g6h fix: resolve nil pointer dereference
  i7j8k9l docs: update API documentation
```

---

## Tips and Tricks

### Keyboard Shortcuts

- **Ctrl+C**: Cancel current operation
- **Ctrl+D**: Exit (same as `/quit`)
- **Up/Down**: Navigate command history
- **Tab**: Complete slash commands
- **Alt+Enter**: Insert newline in input

### Vim Mode

Enable vim-style editing:

1. Type `/vim` to toggle vim mode
2. Press `Esc` to enter normal mode
3. Use `i` to enter insert mode
4. Use `:` for commands

### Working with Large Files

For files larger than the context window:

```
> Read only the first 50 lines of main.go

[Using tool: bash]
head -n 50 main.go
```

### Multi-line Input

Press **Alt+Enter** (or **Shift+Enter**) to insert newlines:

```
> Write a function that:
  [Alt+Enter]
  - Takes a string parameter
  [Alt+Enter]
  - Returns the reversed string
```

### Using Plan Mode

For complex multi-step tasks:

```
> /plan

Entering plan mode. Describe what you want to do:

> I want to refactor this codebase to use dependency injection

I'll create a plan for refactoring to use dependency injection:

1. Analyze current structure
2. Identify dependencies
3. Create interfaces
4. Implement DI container
5. Update main.go

Approve this plan? (y/n)
```

### Custom Skills

Create project-specific skills in `.agent-harness/skills/`:

```markdown
<!-- .agent-harness/skills/my-project.md -->
# My Project Guidelines

When working with this codebase:
- Use dependency injection
- Follow the repository pattern
- Write tests for all new code
```

### Quick Aliases

Add to your `.bashrc` or `.zshrc`:

```bash
# Quick start in current directory
alias ah='agent-harness'

# Start with read-only mode
alias ahr='agent-harness --permission-mode read-only'

# Start with specific model
alias ah4='agent-harness --model gpt-4o'
```

---

## Best Practices

1. **Start with read-only mode** when exploring unfamiliar code
2. **Compact sessions** regularly to manage token usage
3. **Export important sessions** before clearing
4. **Use `/diff`** to review changes before committing
5. **Set up project config** for consistent team behavior
6. **Enable vim mode** if you prefer modal editing

---

## Getting Help

```
> /help

Available commands:

Session:
  /help          Show available commands
  /status        Show session + workspace context
  /clear         Clear session history
  /compact       Compact session to reduce token usage

Settings:
  /model         Show or change the current model
  /permissions   Show or change permission mode
  /config        Show configuration
  /memory        Show memory context

Output:
  /cost          Show token usage and cost
  /diff          Show git diff
  /export        Export conversation to file
  /version       Show version information
```
