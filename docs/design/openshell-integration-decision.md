# Decision: OpenShell as an external runtime, no Sentry integration

**Status:** accepted for 0.3.40.
**Date:** 2026-10-21.
**Supersedes:** the accepted-risk note in [security review](../security-review.md)
that `bash` can do anything the user can, for users who choose this path.

## Context

An agent harness that executes model-authored shell commands cannot be
secure by prompt or in-process policy alone. Our permission modes and
approval dialog are UX controls inside the agent's own process; a
compromised or prompt-injected agent can work around them. The 0.3.39
review recorded that as an accepted risk and named out-of-process
enforcement as the structural fix.

Two NVIDIA projects are relevant:

- **OpenShell** is an Apache-2.0 runtime (v0.1.5 as of October 2026) that
  starts an agent inside a sandbox and enforces filesystem, process, and
  network policy outside the agent process. It ships a Go SDK, a CLI, a
  policy YAML schema, and provider profiles that keep credentials outside
  the workload. It requires Linux, macOS on Apple Silicon, or WSL 2, plus
  Docker, Podman, or a VM runtime.
- **Sentry** is not open source and not downloadable. It is a reference
  system design that runs on BlueField-4 DPUs through DOCA, with no public
  repository, SDK, price, or ship date. NVIDIA describes the DOCA pieces
  as available for experimentation "ahead of NVIDIA Sentry's planned
  availability early next year."

The harness also had a documentation gap: `docs/install.md` promised
Docker images and `docs/architecture.md` described a `Dockerfile` that did
not exist. OpenShell needs an OCI image containing the harness, so fixing
that gap is a prerequisite either way.

## Decision

1. Ship an official multi-arch image and publish it to GHCR on release
   tags.
2. Document OpenShell as an optional runtime boundary, with a restrictive
   policy and an OpenRouter provider profile as examples.
3. Do **not** add the OpenShell Go SDK as a dependency in 0.3.40.
4. Do **not** claim or implement Sentry support.

## Why this shape

| | External runtime (chosen) | Embedded Go SDK (deferred) | Sentry (rejected) |
|---|---|---|---|
| Work | Image, workflow, docs, policy | 2–3 weeks: gateway auth, sandbox lifecycle, exec streaming, file sync, tests | Not possible |
| Risk | None to the tool-execution path | Touches the path we require to work flawlessly; pre-1.0 SDK with a documented breaking migration | None, because there is no software to integrate |
| User value | Isolation for `bash`, egress allowlists, credential brokering, audit | Same, plus in-TUI control that duplicates the OpenShell CLI | Only for operators with BlueField-4 hardware |
| Platform | Linux, macOS arm64, WSL 2 | Same | Datacenter DPUs |

Embedding the SDK would also force a choice between two bad execution
models: run only `bash` in the sandbox while file tools stay on the host,
which gives the agent two divergent filesystems, or run the whole harness
in the sandbox, in which case the SDK is just a launcher around the CLI.
Neither improves on the documented workflow enough to justify the
regression surface in a patch release.

Sentry's portable artifact is the policy, not code. Its reference
architecture evaluates OpenShell/OPA policies, so the policy files shipped
here are the part that can carry forward if a user later deploys the
hardware.

## Consequences

Positive:

- Users get a real out-of-process boundary today, with no change to the
  direct-binary path.
- The image fixes stale Docker documentation and gives every future
  sandbox runtime a build artifact.
- Credentials can stay outside the sandbox through provider placeholders.

Negative:

- OpenShell is 0.x; its CLI and profile schema can change between minor
  releases, so the docs need revalidation each cycle.
- There is no live workspace mount in 0.1.x; sessions need explicit upload
  and download.
- Termux and native Windows users cannot use this path.
- Two policy systems now coexist: harness permissions and OpenShell
  policy. The docs state that they compose rather than replace each other.

## Alternatives considered

- **Do nothing.** Keeps the accepted risk and the stale Docker docs.
- **Generate OpenShell policy from harness permission settings.** The two
  models do not map cleanly (tool-level approvals versus kernel paths and
  binary-bound egress), and a generated policy would invite users to trust
  it without review.
- **Bundle OpenShell as a dependency or installer.** It needs
  Docker/Podman/VM and a gateway; bundling turns an optional boundary into
  a mandatory runtime dependency.

## Revisit when

- OpenShell reaches 1.0 and its SDK API stabilizes.
- Users ask for sandbox lifecycle control inside the TUI.
- OpenShell adds a live workspace mount, which removes the upload/download
  friction.
- Sentry becomes downloadable and a user has BlueField-4 hardware and a
  concrete deployment.

## Evidence

- [NVIDIA/OpenShell](https://github.com/NVIDIA/OpenShell): Apache-2.0,
  runtime architecture, policy and provider documentation.
- [OpenShell Go SDK](https://github.com/NVIDIA/OpenShell/blob/main/sdk/go/README.md):
  pre-1.0, gateway-oriented, includes a fake client.
- [OpenShell policy schema](https://docs.nvidia.com/openshell/latest/how-it-works/policies/schema):
  filesystem, Landlock, process, and network fields used by the examples.
- [OpenShell provider profiles](https://docs.nvidia.com/openshell/latest/how-it-works/providers/profiles):
  static credential placeholders and endpoint binding.
- [Inside NVIDIA Sentry](https://developer.nvidia.com/blog/inside-nvidia-sentry-secure-ai-agents-at-runtime/):
  DOCA/BlueField-4 design and the "planned availability early next year"
  statement.
