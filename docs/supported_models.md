# Supported Models

> **Note:** Not all models work well with agent-harness. This document tracks tested models and known issues.

---

## Local GGUF

The default checked-in configuration uses a local OpenAI-compatible endpoint,
with DeepReinforce Ornith-1.0 GGUF as the intended local model.

### Recommended Local Model

| Model | Runtime | Quantization | Notes |
|-------|---------|--------------|-------|
| `deepreinforce-ai/Ornith-1.0-9B-GGUF` | `llama.cpp` | `Q4_K_M` | Default local-first path in `agent-harness.yml` |

### Configuration

```bash
agent-harness --diagnose
llama-server -m ./models/ornith-1.0-9b-Q4_K_M.gguf -c 8192 --host 127.0.0.1 --port 8080
agent-harness
```

See [Local Model Setup](local-models.md) for model download, root YAML, and
environment override details.

---

## NVIDIA

NVIDIA's hosted API (`https://integrate.api.nvidia.com/v1`) offers a
generous free tier for learning and demo work. Set the provider to
`nvidia` and authenticate with an `nvapi-...` key via
`NVIDIA_API_KEY` (or `/login` / the setup wizard).

### Recommended Models

| Model | Status | Notes |
|-------|--------|-------|
| `nvidia/nemotron-3.5-lightning-30b-a3b` | ✅ Default | Thinking-capable, great tool use, free tier |
| `nvidia/nemotron-3-super-120b-a12b` | ✅ Supported | Strong reasoning |
| `nvidia/llama-3.1-nemotron-ultra-253b-v1` | ✅ Supported | Largest hosted Nemotron |

Reasoning effort profiles (`/effort`) map to NVIDIA's `reasoning_budget`
(thinking) via `extra_body`; the harness ignores `reasoning_content`
deltas in the stream.

---

## OpenRouter

OpenRouter provides access to multiple hosted model providers through a single
API. It is supported as a hosted alternative when a local model is not
available.

### Recommended Models

| Model | Status | Notes |
|-------|--------|-------|
| `nvidia/nemotron-3-super-120b-a12b:free` | ✅ Default | Excellent tool use, fast, free tier available |
| `anthropic/claude-3.5-sonnet` | ✅ Supported | Strong reasoning, reliable tool execution |
| `anthropic/claude-3.5-sonnet-20241022` | ✅ Supported | Specific version pinning |
| `openai/gpt-4o` | ✅ Supported | Good general purpose model |
| `openai/gpt-4o-mini` | ✅ Supported | Faster, cheaper alternative |

### Known Issues

| Model | Status | Issue |
|-------|--------|-------|
| `qwen/qwen3.6-plus:free` | ❌ Not working | Does not properly handle tool/function calling format |

### Free Tier Models

OpenRouter offers free tier access to many models (suffixed with `:free`). These work well for testing and light usage:

- `nvidia/nemotron-3-super-120b-a12b:free` - Recommended starting point
- `google/gemini-2.0-flash-exp:free` - Fast responses
- `deepseek/deepseek-chat:free` - Good reasoning

**Note:** Free tiers typically have rate limits. For production use, consider upgrading.

---

## Anthropic

Direct Anthropic API access for Claude models.

### Supported Models

| Model | Status | Notes |
|-------|--------|-------|
| `claude-3-5-sonnet-20241022` | ✅ Supported | Best tool use performance |
| `claude-3-opus-20240229` | ✅ Supported | Most capable, highest cost |
| `claude-3-5-haiku-20241022` | ⚠️ Limited | Faster but less capable for complex tools |

### Configuration

```bash
export ANTHROPIC_API_KEY="your-key"
export AGENT_HARNESS_PROVIDER="anthropic"
export AGENT_HARNESS_MODEL="claude-3-5-sonnet-20241022"
```

---

## OpenAI

Direct OpenAI API access.

### Supported Models

| Model | Status | Notes |
|-------|--------|-------|
| `gpt-4o` | ✅ Supported | Good balance of capability and speed |
| `gpt-4o-mini` | ✅ Supported | Cost-effective for simple tasks |
| `gpt-4-turbo` | ✅ Supported | Older but still capable |
| `gpt-3.5-turbo` | ⚠️ Limited | May struggle with complex tool sequences |

### Configuration

```bash
export OPENAI_API_KEY="your-key"
export AGENT_HARNESS_PROVIDER="openai"
export AGENT_HARNESS_MODEL="gpt-4o"
```

---

## Fireworks AI

OpenAI-compatible serverless inference for open-source models, via
`https://api.fireworks.ai/inference/v1` (Bearer key). Model IDs use the
`accounts/fireworks/models/<name>` form; `/models` lists the full catalog
in the model picker.

### Recommended Models

| Model | Status | Notes |
|-------|--------|-------|
| `accounts/fireworks/models/llama-v3p3-70b-instruct` | ✅ Default | Solid all-round tool use |
| `accounts/fireworks/models/deepseek-v4-flash-0731` | ✅ Supported | Fast reasoning-capable flash model |
| `accounts/fireworks/models/mixtral-8x22b-instruct` | ✅ Supported | High-throughput mixture of experts |

### Configuration

```bash
export AGENT_HARNESS_PROVIDER="fireworks"
export AGENT_HARNESS_MODEL="accounts/fireworks/models/llama-v3p3-70b-instruct"
```

Or run `/login` and pick Fireworks (the key is stored encrypted).

---

## NVIDIA

NVIDIA NIM build.nvidia.com endpoint, OpenAI-compatible at
`https://integrate.api.nvidia.com/v1` (Bearer key). Nemotron models include
a free tier (rate-limited).

### Recommended Models

| Model | Status | Notes |
|-------|--------|-------|
| `nvidia/nemotron-3.5-lightning` | ✅ Default | Fast, good tool use |
| `nvidia/nemotron-3.5-lightning:free` | ⚠️ Free tier | Rate-limited |

### Configuration

```bash
export AGENT_HARNESS_PROVIDER="nvidia"
export AGENT_HARNESS_MODEL="nvidia/nemotron-3.5-lightning"
```

Or run `/login` and pick NVIDIA (the key is stored encrypted).

---

## Model Selection Guide

### For Development/Testing
```bash
# Local GGUF default
deepreinforce-ai/Ornith-1.0-9B-GGUF

# Lightweight Ollama fallback
gemma4:2b
```

### For Serious Work
```bash
# Local-first
deepreinforce-ai/Ornith-1.0-9B-GGUF

# Best overall performance (OpenRouter)
anthropic/claude-3.5-sonnet

# Best overall performance (Direct)
claude-3-5-sonnet-20241022
```

### For Speed/Cost
```bash
# OpenRouter
openai/gpt-4o-mini

# OpenAI direct
gpt-4o-mini
```

---

## Changing Models

### Via Root YAML

Edit `agent-harness.yml`:

```yaml
provider: local
runtime: llama.cpp
model: deepreinforce-ai/Ornith-1.0-9B-GGUF
model_path: ./models/ornith-1.0-9b-Q4_K_M.gguf
endpoint_url: http://127.0.0.1:8080/v1
```

### Via Environment Variable

```bash
export AH_MODEL="deepreinforce-ai/Ornith-1.0-9B-GGUF"
agent-harness
```

### Via Slash Command (TUI)

```
/model nvidia/nemotron-3-super-120b-a12b:free
```

### Via Settings Tab (TUI)

1. Press `Tab` to switch to Settings view
2. Navigate to "Model" setting
3. Enter new model name
4. Press Enter to save

---

## Troubleshooting

### Model Not Responding

1. Check if the model supports function/tool calling
2. Verify your API key has access to the model
3. Try a different model from the recommended list

### Tool Execution Failures

Some models struggle with complex tool sequences. If you see:
- Failed tool calls
- Empty responses after tool use
- Incorrect tool parameters

Switch to a recommended model like `nvidia/nemotron-3-super-120b-a12b:free` or `claude-3.5-sonnet`.

---

## Contributing

If you test a model not listed here, please open an issue with:
- Model name
- Provider/runtime (local/llama.cpp, Ollama, OpenRouter, Anthropic, OpenAI)
- Status (working/not working)
- Any notes about performance or issues
