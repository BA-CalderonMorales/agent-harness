# Security review

A review of the shipping code as of 0.4.0, with the fixes that came out
of it and the risks that remain by design. Read this with
[credentials](credentials.md) and [command approval](command-approval.md):
those describe intended behavior, this describes where it can fail.

## Scope and method

- Source review of `internal/` and `cmd/`, focused on the paths that
  consume untrusted input: model-authored tool arguments, tool output,
  MCP servers, project configuration, and exported session files.
- `govulncheck ./...` against the Go vulnerability database, plus a
  manual dependency and toolchain check.
- Targeted source searches for the usual Go failure shapes: `exec` with
  shell interpolation, unbounded `io.ReadAll`, HTTP clients without
  timeouts, credential file modes, and path handling in file tools.
- Every fix below is covered by a regression test and by the existing
  suite (`go test ./...`).

Not covered: dynamic analysis, fuzzing beyond the existing property
tests, and the remote services behind a user's provider endpoint.

## Threat model

The harness runs with the user's privileges and executes model-chosen
shell commands. It therefore cannot protect the machine from a model or
user that is determined to run code. What it can do is refuse silent,
unintended exposure: secrets in files that leave the machine, network
requests the user did not ask for, files that leak credentials through
`/export`, and tool output that manipulates the terminal.

Project-scoped configuration is executable content. A cloned
repository's `agent-harness.yml` chooses the provider, model, and
endpoint, and its `.mcp.json` names commands the harness starts. Treat
opening an untrusted repository the same way you would treat running
its test suite. This is recorded as an accepted risk below rather than
silently assumed.

## 0.4.0 follow-up

The release-branch review re-ran the toolchain scan and the terminal
filter checks against the 0.4.0 changes:

- GO-2026-6809 (distrust compromised intermediate certificates in
  `crypto/x509`) was reachable on the 1.26.9 line. The module, CI,
  release, Windows, Docker, and install requirements now say Go 1.26.10,
  and `x/net` moves to v0.62.0 for GO-2026-6860 and GO-2026-6866.
- The live tool-start path built its row from model-authored arguments
  without the S-4 filter, and status flashes stored their text before
  filtering. Both now go through `SanitizeTerminalText`; regression tests
  cover the OSC 52 payloads.
- The new `evals/` helper launches the app and the independent test
  subprocess with a scrubbed environment. Host provider and cloud
  credentials are not part of a local-model case.

## November 4 release audit

The final review includes the uncommitted terminal evaluation/campaign changes,
not only the earlier provider and TUI commits. Regression checks now cover:

- Host verification copies only bounded regular files, rejects symlinks,
  FIFOs and oversized snapshots, and runs candidate code with no network.
- Verification stdout/stderr share a 1 MiB capture budget and a deadline;
  process groups are killed and reaped on failure or excessive output.
- Observation records have a validated schema, a 1 MiB per-record limit and
  a 32 MiB stream limit. Records remain untrusted claims, never proof that a
  task succeeded. Independent host checks decide objective outcomes.
- Worker mounts exclude host evidence and grading answers; private grading
  uses a fresh filesystem view. Scripted-provider tests exercise the real TUI,
  cancellation, follow-up, approval handling, and attempts to alter evidence.
- Shell failures propagate the tool-error flag instead of masquerading as
  successful test runs. CI now runs the Linux terminal/security regressions.

The full Go suite passes with the race detector. The MCP test peer disables
only the race runtime's artificial exit delay, which otherwise collided with
its transport shutdown deadline; race detection remains enabled.

The Go vulnerability scan reports no reachable findings. GO-2026-5932 is
module-only in `x/crypto` v0.58.0; the database names a prerelease fix, while
v0.58.0 remains the latest stable version checked on November 4. Optional
GitNexus tooling has zero high/critical and three moderate npm audit findings,
with no compatible fix offered. That JavaScript tooling is not shipped in
the Go executable. Recheck both exceptions on the next dependency update.

These checks do not establish general task reliability. Campaigns screen
bounded fixtures, retain inconclusive results, and do not deploy changes.
Sampled process-tree limits exclude the separate model server and are not
hard container-wide memory guarantees. The worker retains provider network
access; this is not an adversarial egress boundary. Existing workspace-trust
and command-approval limitations below still apply.

## Findings

| ID | Severity | Finding | Status |
|---|---|---|---|
| S-1 | High | `web_fetch` allowed SSRF to loopback, private, and link-local hosts, followed redirects without rechecking, and read an unbounded body | Fixed |
| S-2 | High | Reachable standard-library and `x/net` advisories (HTTP/2 framing, flow control, TLS ECH, MIME parsing) in the pinned toolchain | Fixed |
| S-3 | High | Project-scoped config can execute commands and read local files at boot through `.mcp.json` and `secret://cmd:`/`secret://file:` | Accepted, see below |
| S-4 | Medium | Terminal escape sequences in model or tool output are rendered without filtering | Fixed |
| S-5 | Medium | Destructive-command detection is substring matching and is easy to bypass | Accepted, see below |
| S-6 | Low | `write` and `edit` staged through a predictable `<path>.tmp` file | Fixed |
| S-7 | Low | Credential files did not repair loose permissions and did not reject symlinks | Fixed |
| S-8 | Low | `/export` redaction missed authorization headers, private keys, secret references, and several token formats | Fixed |

### S-1: web_fetch accepted any http-prefixed string

`web_fetch` validated with `strings.HasPrefix(url, "http")`, so `httpfoo://`
passed, and the request itself had no destination policy. A model-authored
URL could reach a local admin panel, a cloud metadata endpoint
(`169.254.169.254`), or any host on the user's LAN, and a hostile server
could stream until memory ran out.

The tool now rejects non-HTTP schemes, missing hosts, and embedded
credentials before the request; resolves the host and refuses loopback,
private, link-local, CGNAT, and multicast addresses; dials the resolved
address so a second DNS answer cannot swap in a private target; stops
after five redirects; and reads at most 1 MiB before applying the same
10k-character window as before. Guarded fetches also ignore an
environment HTTP proxy: the dialer would otherwise vet the proxy rather
than the destination, and a proxy can forward a redirect to a private
address.

Local and private destinations remain available on purpose:

```bash
AGENT_HARNESS_ALLOW_PRIVATE_FETCH=1
```

Non-2xx responses still return their status and body, so callers see no
change there.

### S-2: pinned toolchain carried reachable advisories

`govulncheck` reported nine reachable standard-library advisories fixed in
go1.26.9 (HTTP/2 framing and flow control, HTTP/1 CONNECT
desynchronization, TLS ECH handling, MIME header parsing), five of which
also affected `golang.org/x/net` v0.57.0 and are fixed in v0.60.0.

The 0.4.0 re-run found one newer advisory on that line: GO-2026-6809,
reachable through `crypto/x509` certificate verification and fixed in
go1.26.10. The module, CI, release, Windows, and Docker declarations and
the install docs now say 1.26.10; `x/net` is v0.62.0, which also clears
GO-2026-6860 and GO-2026-6866. A rerun of `govulncheck ./...` reports
zero reachable vulnerabilities. CI still tolerates exit status 3
(findings in unreachable code only, such as the unfixed `x/crypto`
openpgp advisory), which is what keeps unrelated advisories from failing
a release.

### S-3: project configuration is executable content

Two paths make a tracked repository able to run code or read files when
the harness starts in it:

- `cmd/agent-harness/tools_init.go` connects every server in the merged
  MCP configuration, including the project `.mcp.json`, before the user
  approves anything. The server command is arbitrary.
- The project `agent-harness.yml` may set `api_key` to
  `secret://cmd:...` (runs a shell command) or `secret://file:...`
  (reads any file the user can read), and may set `endpoint_url`. Those
  compose: a tracked file can name an attacker endpoint and point the
  key reference at a local secret file.

This is intentional today and documented as a feature in
[credentials](credentials.md), so 0.3.39 documents the risk instead of
changing the default mid-patch. The recommended follow-up, for a minor
release, is a per-repository trust decision:

- Project-scoped servers and secret references start disabled or prompt
  once, with the decision stored per absolute path, the way editors gate
  workspace trust.
- The prompt names the exact command or path, not just the server name.
- `--trust-repo` (or an equivalent settings key) makes the decision
  explicit for automation.

Until then: review `agent-harness.yml` and `.mcp.json` before opening a
repository you did not write, and prefer user-level settings for
credentials.

### S-4: terminal escape sequences are not filtered

Transcript text reaches the terminal through lipgloss and glamour, which
style their own output but do not strip control characters from model or
tool text. A crafted response can emit OSC 52 (clipboard write), OSC 8
(hyperlink), or cursor/erase sequences. A shell tool already lets a
determined caller do anything, so this matters mainly for content the
user asked the model to fetch.

Fixed in 0.3.41. One filter now sits at every ingestion point:
streaming replies, restored sessions, tool rows, log entries, the
approval dialog, and the question modal. It keeps SGR so colored `git`
and test output still look colored, and drops OSC, DCS, PM, APC, SOS,
non-SGR CSI, stray C0 controls, and the C1 range. The question and its
options are filtered before the user sees them, so the answer the model
receives is the answer that was shown. The 0.4.0 review closed the
live tool-start row and status flashes, which had bypassed that filter.

### S-5: destructive-command detection is advisory

`detectDestructive` and `isDangerousCommand` match a handful of
substrings (`rm -rf`, `dd if=`, `mkfs`, `> /dev/`). `rm -fr`, quoting,
shell variables, or a script wrapper bypasses them. They are a warning
layer, not a boundary: the permission engine and the approval dialog are
the real gate, and `bash` is classified destructive so it always asks
outside of explicitly permissive modes.

Tightening the matcher without breaking legitimate commands is a
false-positive minefield (`git commit -m "fix rm -rf handling"` already
matches). Recommended follow-up: parse the command with a shell lexer,
classify the binary and flags, and keep the substring list as a fast
pre-filter. That is a feature-sized change, not a patch.

### S-6, S-7, S-8: fixed in this release

- `write` and `edit` shared a predictable temp path; both now use a
  unique temp file in the destination directory and rename into place,
  which also replaces rather than follows a symlink at the destination.
- Credential loads now refuse a symlinked store and repair a loose file
  mode to 0600; saving tightens a loose config directory to 0700.
- Export redaction now covers `Authorization: ... Bearer`, `x-api-key`,
  `x-auth-token`, private key blocks, `secret://` references, and the
  `xai-`, `AIza`, `hf_`, `glpat-`, `npm_`, `pypi-`, and `sk_live_`
  formats, on top of the existing key/value and path rules.

## Accepted risks

| Risk | Why it is accepted |
|---|---|
| `master.key` sits beside `credentials.enc` | The store protects against backups, sync, and accidental sharing, not a local attacker who already reads the user's home directory. A passphrase prompt was removed deliberately for TUI ergonomics. |
| MCP subprocesses inherit the full environment | Servers are user-installed programs. Scrubbing the environment would break legitimate servers that read `PATH`, proxy, or cloud credentials; the project-config trust decision above is the control. |
| `secret://file:` can read any readable file | Same trust boundary as `secret://cmd:`. Restricting paths would break `/run/secrets` and mounted-secret layouts. |
| `bash` can do anything the user can | The agent is a coding tool. Read-only and plan modes plus the approval dialog are the guardrails. |
| Release artifacts are unsigned | Checksums are published by CD; signing and provenance attestation are a release-engineering follow-up. |

### Out-of-process containment

Every accepted risk above assumes the harness enforces its own boundary.
Users who need a boundary the agent cannot prompt away can run the harness
under [NVIDIA OpenShell](openshell.md): a kernel-enforced sandbox with
Landlock filesystem rules, seccomp and an unprivileged user, and
default-deny egress bound to specific binaries. Provider profiles keep
API keys outside the sandbox and inject them only at approved endpoints.

That confines the accepted risks rather than deleting them. An agent may
still do anything the operator's policy allows, project configuration
still executes inside the sandbox, and the approval dialog still matters
as a UX control. The difference is that a compromised agent can no longer
reach the host filesystem, the user's credentials, or an arbitrary
destination. See [Running under OpenShell](openshell.md) and the
[decision record](design/openshell-integration-decision.md).

## Verifying the fixes

```bash
go build ./... && go vet ./...
go test ./internal/runtime/tools/builtin/ ./internal/core/config/ ./internal/core/state/
govulncheck ./...
```

`make verify` runs the structure check and the full suite. The fetch
guard uses `httptest` servers, so the tests exercise the real dialer and
redirect policy rather than a mock.

## Re-running the review

1. `govulncheck ./...` and compare against the changes in `go.mod`.
2. Re-read the tool constructors in `internal/runtime/tools/builtin/`
   for new outbound I/O, process execution, or file staging.
3. Check `/export` fixtures for new credential shapes when a provider is
   added.
4. Keep this document's status column honest: a fixed finding names the
   release that fixed it, and an accepted risk names the release that
   should revisit it.
