# Credentials & Secrets

## Where keys are safeguarded

agent-harness never keeps API keys in plaintext by default:

- **Encrypted credential store** — `~/.config/agent-harness/credentials.enc`
  (AES-256-GCM, Argon2id key derivation, file mode `0600`, directory mode
  `0700`). Set or update it with `/login`; the key input is hidden and
  never written to chat history or session files.
- **Plaintext-free config** — `agent-harness.yml` and user settings
  (`~/.agent-harness/settings.json`) never contain key material.
- **Environment variables** — `AH_API_KEY` / `AGENT_HARNESS_API_KEY`
  (and `NVIDIA_API_KEY` for the nvidia provider) are read at boot and
  never persisted.

`/config` shows the credential store path and a masked key preview
(`abcd...wxyz`) — never the key itself. `/export` writes conversation
transcripts only; keys are never part of them.

## Sourcing keys from a secrets manager

Any config value for `api_key` may be a `secret://` reference resolved
at boot. Plain values pass through unchanged; references are resolved
through one of three pluggable backends:

| Reference | Resolves to |
|-----------|-------------|
| `secret://env:NAME` | the value of the `NAME` environment variable |
| `secret://file:PATH` | the first non-empty line of the file at `PATH` |
| `secret://cmd:COMMAND` | the first non-empty line of `COMMAND`'s stdout |

The `cmd` backend wraps any external manager without native SDKs, so the
harness stays provider-agnostic:

```yaml
# agent-harness.yml
provider: openrouter
api_key: "secret://cmd:gcloud secrets versions access latest --secret=openai-key"
```

```yaml
# ansible-vault
api_key: "secret://cmd:ansible-vault view secrets.yml | sed -n 's/^openai_key: //p'"
```

```yaml
# coder secrets
api_key: "secret://cmd:coder secrets show openai-key"
```

```yaml
# docker/k8s mounted secrets
api_key: "secret://file:/run/secrets/openai-api-key"
```

```yaml
# plain environment indirection
api_key: "secret://env:OPENAI_API_KEY"
```

### Rules

- A reference that fails to resolve (unknown backend, missing file,
  failing command) makes the provider misconfigured at boot — the
  harness never sends a literal `secret://...` string as a key, and
  never silently falls back.
- `/config key` accepts **references only**. Literal keys are rejected
  because the command text would land in chat history; use `/login`
  instead.
- The `cmd` backend runs through `sh -c` — treat the command string as
  code and keep it in config you control.
