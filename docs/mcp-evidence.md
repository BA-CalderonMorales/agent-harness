# MCP evidence and lifecycle

An MCP tool is useful only if its result can be trusted to mean what the
server reported. This release repairs the boundary from a subprocess through
approval, execution, model context, and a saved session.

## Design and reference material

The raw reference is [REA's source at 2c4338d](https://github.com/morluto/rea/tree/2c4338d),
which declares 6.6.0. In particular, its tool-result encoding preserves
structured evidence, provenance, limitations, and actionable errors. Its
completion ledger distinguishes verified evidence from unsupported or
incomplete work. These are useful correctness requirements for a harness;
copying REA's tool catalog would not establish them.

The implementation uses the [official MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk/tree/v1.8.0)
for initialization, JSON-RPC framing, request correlation, cancellation, and
stdio process ownership. This follows the same established SDK-based direction
visible in [OpenCode's MCP integration](https://github.com/anomalyco/opencode/blob/dev/packages/opencode/src/mcp/index.ts).
The comparison concerns this boundary, not feature parity with other agents.

## Observable behavior

- Connect starts the configured process, negotiates the protocol, sends
  `notifications/initialized`, and follows every tool-catalog page. Invalid
  names, repeated cursors, and catalogs exceeding 10,000 tools fail setup
  explicitly. Each server has a 30-second setup deadline.
- One failed server produces a startup warning while healthy tools remain
  registered. A failed setup releases its name for retry. A dead connection
  stops advertising its tools through the manager; a previously registered
  wrapper reports that the server is disconnected when called.
- Calls correlate responses by request identity. Notifications and late
  replies cannot stand in for another tool's result. Caller deadlines are
  honored; calls without one receive a two-minute deadline.
- External tools retain their approval checkpoint. A server's `readOnlyHint`
  does not grant permission. Roots, sampling, and elicitation are not advertised.
- Simple text blocks are separated by newlines. Structured content, metadata,
  resources, media, and tool errors retain their complete MCP envelope as JSON.
  This preserves media references/data; it does not add multimodal rendering.
- A tool's `isError` remains an error in the model request, audit outcome, TUI
  tool status, and saved session. Oversized results use the existing bounded
  receipt with a SHA-256 address and private, complete result file. Receipt
  substitution cannot turn a failure into a success.
- Progress with a matching token becomes sanitized, temporary TUI status.
  Progress is advisory and is not saved as assistant speech or evidence.
- Normal application exit and signal shutdown close the manager. Pending
  setup is cancelled; owned command transports are closed and reaped.

Only stdio transport is supported. HTTP transport, automatic reconnection,
dynamic catalog refresh, and interactive server requests remain outside this
change. Unsupported interaction results are reported as failures. Server
stderr is discarded rather than mixed with protocol output or retained in an
unbounded log. Startup and protocol errors still identify the server.

## Reproduce the independent check

The ordinary test suite is offline. Its raw JSON-RPC subprocess fixture uses
no MCP SDK, so malformed startup, pagination, notifications, cancellation,
late replies, concurrent calls, and process exit exercise the production
client from the other side of the wire. Agent-loop tests cover both streaming
and batch execution, approval denial, budget receipts, model continuation,
and session reload.

The opt-in live check needs Go, Node, npm, and network access on the first run:

```sh
go build -o /tmp/agent-harness-review ./cmd/agent-harness
AGENT_HARNESS_REA_SMOKE_BINARY=/tmp/agent-harness-review \
  go test ./internal/runtime/services/mcp \
  -run '^TestREACompatibilitySmoke$' -v -count=1 -timeout 2m
```

It pins the published `rea-agents@6.5.0`, which is distinct from the newer
source checkout. The check discovers `inspect_go_binary`, submits a missing
target, then inspects the built executable over the same connection. Without
executing the target, it compares the reported SHA-256 and size to the file,
the compiler version to Go's `debug/buildinfo`, and both cited build-info
regions to the exact source bytes. The verified run discovered 207 tools.

To use REA interactively, add a server entry to the project's `.mcp.json`
(merge it with any existing entries):

```json
{
  "mcpServers": {
    "rea": {
      "command": "npx",
      "args": ["--yes", "--package=rea-agents@6.5.0", "rea", "--mcp"]
    }
  }
}
```

Restart the harness and request an inspection of an executable's absolute
path. Review the tool approval, observe progress when supplied, and check
that evidence and limitations survive a session resume. Evidence returned
by the server is still untrusted target content; retaining provenance is not
an independent verification of every server claim.
