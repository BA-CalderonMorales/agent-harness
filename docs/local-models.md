# Local Model Setup

agent-harness is local-first by default. The checked-in `agent-harness.yml`
targets an OpenAI-compatible local server, with DeepReinforce Ornith-1.0 GGUF
as the intended default model.

## Default Runtime

- Provider: `local`
- Runtime: `llama.cpp`
- Model: `deepreinforce-ai/Ornith-1.0-9B-GGUF`
- Suggested quantization: `Q4_K_M`
- Model path: `./models/ornith-1.0-9b-Q4_K_M.gguf`
- Endpoint: `http://127.0.0.1:8080/v1`
- Context length: `8192`

## Configure

The root `agent-harness.yml` is the setup source of truth for a cloned repo.
For installed use, create the same file in the workspace where you run
`agent-harness`, or use the matching environment variables.

```yaml
provider: local
runtime: llama.cpp
model: deepreinforce-ai/Ornith-1.0-9B-GGUF
model_path: ./models/ornith-1.0-9b-Q4_K_M.gguf
endpoint_url: http://127.0.0.1:8080/v1
context_length: 8192
temperature: 0.2
max_tokens: 4096
permission_mode: workspace-write
```

Environment overrides use the `AH_*` or `AGENT_HARNESS_*` prefix:

- `AH_PROVIDER`
- `AH_RUNTIME`
- `AH_MODEL`
- `AH_MODEL_PATH`
- `AH_ENDPOINT_URL`
- `AH_CONTEXT_LENGTH`
- `AH_TEMPERATURE`
- `AH_MAX_TOKENS`
- `AH_WORKSPACE_PATH`
- `AH_LOCAL_SERVER_COMMAND`
- `AH_PERMISSION_MODE`
- `AH_PERM_READ`
- `AH_PERM_WRITE`
- `AH_PERM_DELETE`
- `AH_PERM_EXECUTE`

## Download A GGUF

Place the selected GGUF file under `./models/` or update `model_path`. The
Q4_K_M file published with the Hugging Face repo is named
`ornith-1.0-9b-Q4_K_M.gguf`.

```bash
mkdir -p models
huggingface-cli download deepreinforce-ai/Ornith-1.0-9B-GGUF \
  --include "*Q4_K_M*.gguf" \
  --local-dir models
```

If that quantization is not available for your machine, use another GGUF file
and update `model_path` plus `local_server_command`.

## Start llama.cpp

Use the command from `agent-harness.yml`, adjusted to the actual file name:

```bash
llama-server \
  -m ./models/ornith-1.0-9b-Q4_K_M.gguf \
  -c 8192 \
  --host 127.0.0.1 \
  --port 8080
```

The server must expose an OpenAI-compatible `/v1/chat/completions` endpoint.

## Run

```bash
make build
./build/agent-harness
```

For a quick configuration check:

```bash
./build/agent-harness --diagnose
```

`--diagnose` reports whether `model_path` exists and whether the configured
OpenAI-compatible `/models` endpoint is reachable. These checks do not run
during normal TUI startup.

## Fallbacks

For lightweight smoke testing, use Ollama or another OpenAI-compatible local
runtime by changing `provider`, `runtime`, `model`, and `endpoint_url`.

Example Ollama env override:

```bash
AH_PROVIDER=ollama AH_MODEL=gemma4:2b agent-harness
```
