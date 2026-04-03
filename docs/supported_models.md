# Supported Models

> **Note:** Not all models work well with agent-harness. This document tracks tested models and known issues.

---

## OpenRouter

OpenRouter provides access to multiple model providers through a single API. We default to OpenRouter for its flexibility and free tier options.

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

## Model Selection Guide

### For Development/Testing
```bash
# Free tier on OpenRouter
nvidia/nemotron-3-super-120b-a12b:free
```

### For Serious Work
```bash
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

### Via Environment Variable

```bash
export AGENT_HARNESS_MODEL="nvidia/nemotron-3-super-120b-a12b:free"
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
- Provider (OpenRouter/Anthropic/OpenAI)
- Status (working/not working)
- Any notes about performance or issues
