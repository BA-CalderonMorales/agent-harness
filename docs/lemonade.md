# Lemonade evaluation on the ZBook

Status on 2026-10-27: host packages are installed. User-provided validation
detects the NPU, eight columns, firmware 1.1.2.64, and amdxdna 0.10 with
unlimited memlock. The system service is active on port 13305 with unlimited
soft/hard memlock limits. Qwen3.5-9B-FLM is loaded on the NPU with a 16,384
token context. A user-run text request returned the exact requested answer;
the short sample reported 1.295 seconds TTFT and 7.33 decoding tokens/sec.
This is not a sustained-performance benchmark. The user-run smoke check
passed text (2.12s), tool call (3.16s), tool result (2.39s), and vision (2.24s).
These are request durations, not per-token rates or production-quality
scores. Streaming, real coding tasks, and recovery remain unvalidated.
The agent session cannot elevate privileges or
access the host system service bus or loopback server.
No model or default application configuration was changed by the agent.

The later 2B coding exercise failed acceptance despite fast smoke checks:
it changed the excluded test file and emitted write calls without content.
The resulting implicit truncation exposed a file-tool validation bug,
which now has regression coverage. Keep 2B experimental and evaluate 4B
on the same task after the rebuilt mutation guard is active.

The inspected machine has a Ryzen AI 7 PRO 350, Radeon 860M, about 32 GB
RAM, and an NPU bound to `amdxdna`. A bound driver is not an inference test.
Use [Lemonade's Linux NPU guide](https://lemonade-server.ai/flm_npu_linux.html)
to validate firmware, runtime access, and memory-lock requirements.

## Model candidates

Start with Qwen3.5-9B as the quality-first candidate and Qwen3.5-4B as the
smaller alternative. Both have vision and tool-calling entries in the
[Lemonade catalog](https://lemonade-server.ai/docs/models.html). Compare
Gemma-4-12B-it if the initial model's task accuracy is inadequate. These are
evaluation candidates, not benchmark winners on this machine.

The catalog lists the Qwen3.5 9B GGUF package at about 6.88 GB and the 4B
package at 3.58 GB. Downloads are not peak RAM requirements: allow for
runtime buffers, vision processing, context, and ordinary desktop work.
Avoid starting with the roughly 23 GB Qwen3.6-35B-A3B package on a shared
32 GB desktop. Few active parameters do not eliminate the other weights.

Qwen's [model card](https://huggingface.co/Qwen/Qwen3.5-9B) describes image
understanding and tool use. This recommendation covers text and image
input; it does not promise audio input, speech output, or image generation.

## Install on the host

Run in the host terminal, outside the restricted agent session:

```bash
omarchy pkg add lemonade-server fastflowlm xrt xrt-plugin-amdxdna
sudo systemctl enable --now lemond
sudo systemctl --no-pager status lemond
flm validate
lemonade list
```

The [Arch installation guide](https://lemonade-server.ai/docs/guide/install/arch/)
documents the server and service. Check the available package versions on
the host; this session's package database listed FastFlowLM 1.0.9, while
upstream has released 1.0.10 with tool parsing and constrained-decoding
changes. Follow the host's normal system-update process before evaluating
newer behavior; do not mix arbitrary runtime binaries.

### Memory-lock limit on the installed Arch package

The inspected Arch 2026.42.1 service grants `CAP_SYS_RESOURCE` but does not
explicitly set `LimitMEMLOCK`. Set the limit for Lemonade and its child
processes using the supplied drop-in, from the repository root:

```bash
sudo install -Dm644 examples/lemonade/90-npu-memlock.conf \
  /etc/systemd/system/lemond.service.d/90-npu-memlock.conf
sudo systemctl daemon-reload
sudo systemctl restart lemond
systemctl show lemond -p ActiveState -p LimitMEMLOCK -p LimitMEMLOCKSoft
lemonade status
```

Expect an active service and unlimited soft/hard memlock limits. This is
scoped to the system service, not all users. Restarting interrupts any
active Lemonade work. A successful service check still needs a model test.

Direct `flm validate` inherits the terminal's limit, so it may still report
8 MB after the service is fixed. For a temporary check, raise only the
current shell's limits from the host terminal, then validate as yourself:

```bash
sudo prlimit --pid "$$" --memlock=unlimited:unlimited
flm validate
```

This does not run FLM as root and does not persist across login sessions.
Do not change global login limits just to validate the system service.

## Establish a baseline

### Launch Agent-Harness with the verified NPU profile

From the workspace you want the agent to use:

```bash
bash /home/bmoe/Projects/agent-harness/scripts/ah-lemonade.sh
```

The launcher checks that the endpoint lists the selected model, builds the
current app, and launches it in the caller's workspace. It sets the local
endpoint, model, placeholder key, 8K input budget, and 4K output limit for
that process. It does not install packages, start the service, or edit the
repository's default YAML. The app's normal settings/session persistence
still applies. Use `--help` for profile overrides.

For the first app test, use a disposable Git workspace and ask it to read a
small file, make one specific change, and run its test. Inspect the diff.
Then test cancellation and resume. Server smoke checks do not establish
that the app's streamed tool execution path works with this backend.

### Server checks and GPU comparison

For this installed server, `lemonade config set flm.prefer_system=true`
followed by a service restart exposed the system FastFlowLM backend and
`qwen3.5-9b-FLM` entry. The host successfully pulled that entry and loaded it
with `lemonade load qwen3.5-9b-FLM --ctx-size 16384`. Prefer catalog IDs
confirmed by the installed server. This NPU setup is separate from the
GGUF baseline below.

Run the server smoke checks from any directory:

```bash
python /home/bmoe/Projects/agent-harness/examples/lemonade/smoke.py
```

The script sends text, a harmless synthetic tool round trip, and a generated
solid-color PNG. It neither executes model-chosen commands nor sends real
screenshots. It requires only Python's standard library and reports failures
per capability. It tests the server API, not the TUI or production coding
quality. Use `--skip-vision` to isolate text/tool failures.

Failures remain failures, including extra prose around an exact requested
tool value. Independent checks continue so a formatting failure does not
hide the vision result; a bad tool call skips its dependent result check.
The script still exits nonzero if any check fails. The first user-run 2B
trial passed text (0.70s) and tool calling (1.08s), then returned the correct
tool value with an unwanted prose wrapper. Vision was not reached in that
older fail-fast run. This is evidence of faster short requests and an
exact-instruction miss, not proof of coding quality.

Use the catalog's GGUF entry first to establish API and model compatibility:

```bash
lemonade pull Qwen3.5-9B-GGUF
lemonade load Qwen3.5-9B-GGUF --ctx-size 16384
curl --fail --silent --show-error http://127.0.0.1:13305/v1/models
```

This is the llama.cpp recipe, not evidence of NPU execution. Inspect
Lemonade's backend status and logs to record the actual accelerator. For
the NPU comparison, select the available FastFlowLM entry from the
installed server's catalog. Do not assume a GGUF entry uses the NPU, or
that GGUF and FLM quantizations will produce identical answers.

Build the app normally, then connect using its existing compatible API
path. These overrides do not edit the repository's default configuration:

```bash
go build -o build/agent-harness ./cmd/agent-harness
AH_PROVIDER=local \
AH_RUNTIME=lemonade \
AH_ENDPOINT_URL=http://127.0.0.1:13305/v1 \
AH_API_KEY=lemonade-local \
AH_MODEL=Qwen3.5-9B-GGUF \
AH_CONTEXT_LENGTH=8192 \
AH_MAX_TOKENS=4096 \
./build/agent-harness
```

The placeholder key prevents an inherited hosted-provider key from being
selected for this invocation. Use the actual local key if authentication
is enabled. The server context is deliberately larger than the app's
initial input budget to leave output headroom; verify effective limits.
`--diagnose` currently also checks a local model path, even though Lemonade
owns the model files. That warning is not evidence that Lemonade is broken.

## Multimodal work still required

The current app's content blocks and HTTP serialization support text,
thinking, tool calls, and tool results. They do not yet send images. Test
vision directly through Lemonade first, using its documented
[image request format](https://lemonade-server.ai/docs/api/openai/).

Before exposing vision in the TUI, implement explicit image attachments,
bounded input sizes, MIME validation, API serialization, and a persistence
policy for attachment data. Verify resume and export behavior and preserve
the existing text/tool path. A vision-capable model alone does not enable
multimodal interaction in the app.

## Caching and LMCache

[LMCache](https://github.com/LMCache/LMCache) can retain and reuse inference
state across requests and move it between memory and storage tiers. It
addresses repeated prompt processing; it does not remove model weights or
guarantee that a larger model fits. Cache transfer and storage have costs.

Its [published ROCm installation](https://docs.lmcache.ai/getting_started/installation.html)
targets AMD Instinct GPUs. The
[compatibility guidance](https://docs.lmcache.ai/getting_started/compatibility.html)
requires matching the serving engine, model state layout, device, and
runtime. No validated FastFlowLM/Ryzen XDNA integration was established in
this evaluation. Do not add LMCache as a dependency of the initial setup.

First measure the backend's existing prompt reuse with repeated turns.
Keep tool schemas and system context ordering stable; measure how edits,
compaction, and alternating sub-agents affect reuse. The app currently
iterates a map when appending system context, so stable ordering needs a
code review before relying on repeatable prefixes. Revisit LMCache if a
compatible engine and a measured prefill bottleneck justify it.

## Acceptance evidence

Record the model ID, quantization, runtime version, accelerator, context,
and peak memory for each run. Compare the same fixtures on each backend:

- Cold model load and warm follow-up time to first token.
- A read/edit/test task in a disposable workspace; verify the resulting diff.
- Tool schema correctness and recovery after a tool error.
- Screenshot interpretation through the server, then through the app once
  attachments exist; include a screenshot with small text.
- Cancellation, backend restart, session resume, and a long conversation.
- Desktop responsiveness during inference and concurrent agent work.

Treat failed tasks and model/backend switches as results to report. No
automatic cloud fallback or system-wide runtime changes are part of this
recipe.
