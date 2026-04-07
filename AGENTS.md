# AGENTS.md - Agent Harness

## Quick Reference

- **Source**: `cmd/agent-harness/main.go`
- **Run**: `./scripts/run-termux.sh` or `~/buckets/usr/bin/agent-harness`
- **Local LLM**: `./scripts/ah-fast.sh` (gemma4:2b) or `./scripts/ah-local.sh` (gemma4:4b)

## Cross-Repo

- Related: terminal-jarvis (Rust ADK), lumina-bot (Go gateway), claude-termux (JS CLI)
- Shared commands: `harness-status`, `sync-philosophy`

## Key Patterns

- Tool Descriptor Pattern: structs with function fields, not interfaces
- Permission Stack: deny → allow → ask → mode transforms → tool-specific checks
- File Operations: cache by (path, offset, limit, mtime), stale-write protection, atomic writes

## Security

- UNC paths rejected (prevent NTLM leaks)
- Device paths blocked
- Bash uses `exec.LookPath("sh")` for portability

## Termux

- Build: `go build -o ./build/agent-harness ./cmd/agent-harness`
- Use project-local dirs (not /tmp)
- Shell at `$PREFIX/bin/sh`

## Environment Variables

- `AH_PROVIDER`: openrouter, openai, anthropic, ollama
- `AH_MODEL`: model identifier
- `AH_API_KEY`: API key (not needed for ollama)
- `OLLAMA_HOST`: Ollama server URL (default: http://localhost:11434)

## Testing

- `go test ./...`
- `go test -race ./...`

## Critical Rules

- Zero emojis in root-level .md files
- Lowercase filenames (except README.md, AGENTS.md)
- No horizontal rules as section separators
- Tool calling must work flawlessly - no regressions

## Working Rules

- Stop and explain before major architectural changes
- One change per commit, commit before starting next
- Conventional commits: `type(scope): description`
