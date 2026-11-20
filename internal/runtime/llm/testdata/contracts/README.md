# Provider contract fixtures

Each file is a raw HTTP response: a status line, the headers the client
reads, a blank line, then the body exactly as the server sent it. The
contract tests (`internal/runtime/llm/contract_*_test.go` and
`internal/agent/provider_overflow_contract_test.go`) parse a file with
`http.ReadResponse`, serve it from an `httptest` server, and drive the
real `HTTPClient` against it. No test touches the network.

## Origin of each fixture

| Provider | File | Origin |
|---|---|---|
| lemonade | `models.http` | Captured live |
| lemonade | `stream-text.http` | Captured live |
| lemonade | `stream-tool-call.http` | Captured live |
| lemonade | `overflow.http` | Captured live |
| openrouter | every file | Constructed |
| fireworks | every file | Constructed |

### Captured live

The Lemonade fixtures were recorded by
`scripts/verify/capture-lemonade-contracts.py` from a local Lemonade
2026.42.1 server with `qwen3.5-9b-FLM` loaded at a 4096-token window.
Every prompt was synthetic. To refresh them when Lemonade changes, run the
script with `--out DIR` (it refuses to write into this tree unless
`--overwrite-committed` is passed), review the diff, copy over what changed,
then run `go test ./internal/runtime/llm/ ./internal/agent/`.

Lemonade facts these fixtures pin:

- `/models` reports `context_length` (the window it loaded) next to
  `max_context_window` (what the model supports). The app must budget
  against `context_length`.
- A request past the loaded window is answered with HTTP 200 and one SSE
  line, `{"error":{"code":400,"message":"Max length reached!",...}}`,
  with no `[DONE]`. It is not an HTTP 400.
- A streamed tool call arrives whole in a single chunk.

### Constructed

No OpenRouter or Fireworks key was available where these were written,
so their fixtures are built by hand from the shapes `HTTPClient` already
parses (`sse.go`, `api_errors.go`, `models.go`) and from the existing
unit tests. They carry only the fields the client reads: chunk `id`,
`model`, `choices[0].delta` (`content`, `reasoning` or
`reasoning_content`, `tool_calls`), `finish_reason`, `usage`
(`prompt_tokens`, `completion_tokens`, `prompt_tokens_details.cached_tokens`),
the `error` envelope with OpenRouter's `metadata`, the `X-RateLimit-*`
headers, and `/models` `id` plus `context_length`. Overflow messages use
each provider's known wording; the Fireworks sentence is the least
certain and should be replaced by a capture when a key is at hand.

Tool-call arguments are split across chunks on purpose (mid-key and
mid-value for OpenRouter, two interleaved parallel calls for Fireworks),
because Lemonade's whole-call chunks never exercise accumulation.
