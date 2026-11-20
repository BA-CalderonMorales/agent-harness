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

### Captured live

The Lemonade fixtures were recorded by
`scripts/verify/capture-lemonade-contracts.py` from a local Lemonade
2026.42.1 server with `qwen3.5-9b-FLM` loaded at a 4096-token window.
Every prompt was synthetic. Re-run the script to refresh them when
Lemonade changes, then run `go test ./internal/runtime/llm/ ./internal/agent/`.

Lemonade facts these fixtures pin:

- `/models` reports `context_length` (the window it loaded) next to
  `max_context_window` (what the model supports). The app must budget
  against `context_length`.
- A request past the loaded window is answered with HTTP 200 and one SSE
  line, `{"error":{"code":400,"message":"Max length reached!",...}}`,
  with no `[DONE]`. It is not an HTTP 400.
- A streamed tool call arrives whole in a single chunk.
