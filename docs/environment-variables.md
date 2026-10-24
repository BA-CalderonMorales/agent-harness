# Environment Variables

All variables accept an `AGENT_HARNESS_*` prefix aliasing (e.g.
`AGENT_HARNESS_PROVIDER`), with `AH_*` taking precedence. Environment values
override every config file layer.

- `AH_PROVIDER`: local, ollama, flm, openrouter, openai, anthropic, fireworks, nvidia, omniroute
- `AH_RUNTIME`: llama.cpp, ollama, or another local runtime label
- `AH_MODEL`: model identifier
- `AH_MODEL_PATH`: local GGUF path when using a local runtime
- `AH_ENDPOINT_URL`: OpenAI-compatible API base URL
- `AH_CONTEXT_LENGTH`: model context window
- `AH_TEMPERATURE`: sampling temperature
- `AH_MAX_TOKENS`: maximum response tokens
- `AH_WORKSPACE_PATH`: repository workspace path
- `AH_LOCAL_SERVER_COMMAND`: command users should run for the local server
- `AH_API_KEY`: API key (not needed for local or ollama)
- `AH_PERMISSION_MODE`: read-only, workspace-write, danger-full-access
- `AH_PERSONA`: default persona (developer, designer, pm, scientist, explorer)
- `AGENT_HARNESS_ALLOW_PRIVATE_FETCH`: set to `1` to let `web_fetch` reach
  loopback, private, and link-local hosts (off by default; needed for tests
  and local documentation servers). Guarded fetches connect directly and
  ignore `HTTP_PROXY`/`HTTPS_PROXY`, because a proxy can forward a redirect
  to a private address; opting in restores proxy support
- `AGENT_HARNESS_REDUCED_MOTION`: set to `1` to hold the interface still
  (no shimmer, no movement); `reduced_motion: true` in settings does the
  same. The information stays, only the motion goes
- `OLLAMA_HOST`: Ollama server URL for the fallback Ollama scripts

Advanced settings (e.g. `AGENT_HARNESS_CONFIG_HOME`) are read by
`internal/core/config/`. The config home holds `settings.json`, the
encrypted `credentials.enc`, and its `master.key`; pointing it at a profile
(`AGENT_HARNESS_CONFIG_HOME`, else `XDG_CONFIG_HOME/agent-harness`, else
`~/.config/agent-harness`) carries those credentials with it.
