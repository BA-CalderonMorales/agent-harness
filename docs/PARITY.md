# Feature Parity with claw-code

This document tracks the feature parity between agent-harness and the claw-code reference implementation.

## ✅ Completed Features

### Security & Credentials
- [x] **AES-256-GCM encryption** for API key storage
- [x] **Argon2id key derivation** for master password
- [x] **0600 file permissions** (user read/write only)
- [x] **Password masking** during input (no plaintext echo)
- [x] **Secure migration** from legacy plaintext configs
- [x] **Atomic file writes** to prevent corruption

### Configuration System
- [x] **Layered configuration** (user → project → local)
- [x] **JSON-based config files** with merge semantics
- [x] **Config precedence** respecting source priority
- [x] **Environment variable overrides**
- [x] **Permission modes** (read-only / workspace-write / danger-full-access)
- [x] **Always allow/deny lists** for tool control
- [x] **MCP server configuration** support

### Session Management
- [x] **Persistent sessions** with JSON serialization
- [x] **Session metadata** (ID, timestamps, message count)
- [x] **Auto-save** every 5 turns
- [x] **Session compaction** to reduce token usage
- [x] **Compaction configuration** (max messages, preserve recent)
- [x] **Session listing and loading**

### Slash Commands
- [x] **/help** - Show available commands with descriptions
- [x] **/status** - Show session and workspace context
- [x] **/clear** - Clear session history (with confirmation)
- [x] **/compact** - Compact session to reduce tokens
- [x] **/cost** - Show token usage and estimated cost
- [x] **/model** - Show/change current model
- [x] **/permissions** - Show/change permission mode
- [x] **/config** - Show configuration
- [x] **/diff** - Show git diff of workspace changes
- [x] **/export** - Export conversation to file
- [x] **/session** - List/load saved sessions
- [x] **/version** - Show version information
- [x] **/quit, /exit** - Exit the application

### UI/UX
- [x] **Rich line editor** with bubbletea
- [x] **Command history** (up/down navigation)
- [x] **Tab completion** for slash commands
- [x] **Vim mode support** (normal/insert/visual modes)
- [x] **Formatted output** with lipgloss styles
- [x] **Status reports** with aligned columns
- [x] **Cost reports** with token breakdown
- [x] **Permission reports** with mode descriptions
- [x] **Welcome screen** with system info

### Git Integration
- [x] **Git context detection** (root, branch, commit)
- [x] **Change detection** (uncommitted changes indicator)
- [x] **Git diff** display
- [x] **Project root** identification
- [x] **Remote URL** detection

### Cost Tracking
- [x] **Token usage tracking** per turn
- [x] **Cumulative tracking** across session
- [x] **Cost estimation** per model
- [x] **Model-specific pricing** (Claude, GPT-4, etc.)
- [x] **Formatted reports** with breakdown

### Permission System
- [x] **Three permission modes** with clear semantics
- [x] **Read-only mode** (bash/write/edit blocked)
- [x] **Workspace-write mode** (dangerous tools require confirmation)
- [x] **Danger-full-access mode** (all tools allowed)
- [x] **Always allow/deny lists** for fine-grained control
- [x] **Tool classification** (read-only vs dangerous)

## 🚧 Partially Implemented

### Line Editor
- [ ] Full vim motion support (h/l cursor movement needs custom implementation)
- [ ] Visual mode text selection
- [ ] Yank/paste buffer
- [ ] Command mode (: commands)
- [ ] Multi-line editing with proper cursor positioning

### Session Management
- [ ] Session resume from specific turns
- [ ] Session branching/forking
- [ ] Cloud sync for sessions
- [ ] Session search/filtering

## ⏳ Not Yet Implemented

### Advanced Features
- [ ] **OAuth authentication** with browser flow
- [ ] **Plugin system** with hooks
- [ ] **MCP (Model Context Protocol)** client
- [ ] **LSP integration**
- [ ] **HTTP/SSE server** mode
- [ ] **Sub-agent orchestration**
- [ ] **Plan mode** with multi-step tasks
- [ ] **Context compaction** with summarization
- [ ] **Skills system** with dynamic loading
- [ ] **Agent tool** for delegation
- [ ] **Memory files** (.agent-harness/memory/)
- [ ] **Notebook editing**
- [ ] **Transcript search**

### Commands
- [ ] **/agents** - List/manage sub-agents
- [ ] **/skills** - List/manage skills
- [ ] **/memory** - Show memory context
- [ ] **/init** - Initialize project
- [ ] **/commit** - Git commit helper
- [ ] **/pr** - Pull request helper
- [ ] **/branch** - Branch management
- [ ] **/worktree** - Worktree management
- [ ] **/teleport** - Navigate to file/line
- [ ] **/ultraplan** - Complex planning mode

### Tool Enhancements
- [ ] **Sandboxed bash** execution
- [ ] **File watching** for auto-reload
- [ ] **Image input** support
- [ ] **Web search** with results
- [ ] **Web fetch** with content extraction

### Developer Experience
- [ ] **TUI mode** (full-screen interface)
- [ ] **Syntax highlighting** in output
- [ ] **Markdown rendering** in responses
- [ ] **Streaming output** with live updates
- [ ] **Progress indicators** for long operations
- [ ] **Spinner animations**

## Architecture Decisions

### Similar to claw-code
- Layered configuration with precedence
- Session-based conversation management
- Permission modes with clear semantics
- Slash command system
- Git integration for context

### Different from claw-code
- **Go instead of Rust** - For portability and simpler deployment
- **Bubbletea for TUI** - Instead of custom terminal handling
- **AES-256-GCM for credentials** - Instead of platform keychain (cross-platform)
- **Simpler plugin model** - No dynamic loading yet

## Next Priorities

1. **OAuth authentication** - For cloud-based model access
2. **MCP client** - For external tool integration
3. **Plan mode** - For complex multi-step tasks
4. **Context compaction** - With LLM-based summarization
5. **Skills system** - For reusable prompts
6. **Memory system** - For project-specific context

## Usage Example

```bash
# Start the harness
./agent-harness

# Use slash commands
> /status           # Show session status
> /model gpt-4o     # Switch model
> /permissions      # Show permission mode
> /compact          # Reduce token usage
> /cost             # Show usage and cost

# Regular conversation
> How do I refactor this code?

# Exit
> /quit
```

## Security Notes

- API keys are encrypted with AES-256-GCM
- Master password is required on startup
- File permissions are 0600 (user-only access)
- Legacy configs are automatically migrated and removed
- No plaintext credentials in memory longer than necessary
