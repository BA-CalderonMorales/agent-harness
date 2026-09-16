# Credentials & Secrets

## Where keys are safeguarded

agent-harness never keeps API keys in plaintext by default:

- **Encrypted credential store** — `~/.config/agent-harness/credentials.enc`
  (AES-256-GCM, Argon2id key derivation, file mode `0600`, directory mode
  `0700`). Set or update it with `/login`; the key input is hidden and
  never written to chat history or session files.
- **Plaintext-free config** — `agent-harness.yml` and user settings
  (`~/.config/agent-harness/settings.json`; `AGENT_HARNESS_CONFIG_HOME`,
  else `XDG_CONFIG_HOME`, else `~/.config`) never contain key material.
- **Environment variables** — `AH_API_KEY` / `AGENT_HARNESS_API_KEY`
  (and `NVIDIA_API_KEY` for the nvidia provider) are read at boot and
  never persisted.

`/config` shows the credential store path and a masked key preview
(`abcd...wxyz`) — never the key itself. `/export` writes conversation
transcripts only; keys are never part of them.

## What decides the provider, and which key it uses

Two kinds of knowledge meet at boot, and they answer different
questions. Keeping them separate is what lets a tracked
`agent-harness.yml` ship sane defaults *and* let a returning user keep
their own choice:

| | Question | Home |
|---|---|---|
| **wiring** | which provider/model does this workspace use? | config layers |
| **secrets** | how do I authenticate to provider X? | encrypted store, keyed by provider |

Secrets are keyed *by* provider; the layers *choose* the provider. The
lookup is downstream of the choice, so neither has to outrank the other.

Resolution order for the provider, last writer winning:

1. `AH_PROVIDER` / `AGENT_HARNESS_PROVIDER` — an explicit environment pin.
2. `.agent-harness/settings.local.json` — machine-local, gitignored.
3. `~/.config/agent-harness/settings.json` — what you actually use.
4. `agent-harness.yml` — the tracked project default.
5. The built-in default.

Once the provider is settled, its key is resolved for *that* provider:
an environment pin, then `ProviderKeys[provider]` in the store, then a
config value already authenticating it. A key is never carried across a
provider change — handing one service another's secret is a bug, not a
fallback.

Two rules keep this stable, and both are easy to get wrong in
asymmetric ways:

- **A tracked project file is a default, not a lock.** Your saved setting
  outranks it, which is the whole point: the repo can pin `local` for its
  own development while your machine stays on the provider you chose.
- **Empty is not a choice.** A blank value is never written to a layer
  and never read as an override. Blanking a key looks harmless and is
  not: the reader drops it as a no-op, so the choice is erased rather
  than merely unset, and the next boot falls to the default with no sign
  that anything was lost.

The precedence is pinned by
`cmd/agent-harness/config_precedence_test.go` — if you change how a
provider or key is resolved, that table is the contract to update.

**Known gap: the store's remembered provider does not reach boot.** The
encrypted store also keeps a single "last provider logged into" slot.
It currently never influences the provider, because a provider that
comes only from the built-in default resolves to a local one, and local
providers take their dummy key without consulting the store at all — so
`"no layer chose a provider"` is indistinguishable from `"a layer chose
local"`. A fresh config home with a usable key in the store therefore
boots local. This is pinned by `TestDefaultedLocalShortCircuitsTheStore`
rather than left silent, so closing it means gating that short circuit
on `LayeredConfig.LayerSet("provider")` instead of on the resolved
value, and still handing a genuinely local provider its dummy key
afterwards.

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
