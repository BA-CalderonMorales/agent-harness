# Environment Variables

All variables accept an `AGENT_HARNESS_*` prefix aliasing (e.g.
`AGENT_HARNESS_PROVIDER`), with `AH_*` taking precedence. Environment values
override every config file layer.

- `AH_PROVIDER`: local, ollama, openrouter, openai, anthropic
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
- `OLLAMA_HOST`: Ollama server URL for the fallback Ollama scripts

Advanced settings (e.g. `AGENT_HARNESS_CONFIG_HOME`) are read by
`internal/core/config/`.
