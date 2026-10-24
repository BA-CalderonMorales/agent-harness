# Running under NVIDIA OpenShell

[OpenShell](https://github.com/NVIDIA/OpenShell) is an Apache-2.0 runtime
that puts an agent inside a kernel-enforced sandbox: Landlock for
filesystem access, seccomp and an unprivileged user for process access,
and a default-deny proxy that checks every outbound connection against a
declarative policy. It is not a library we link against — it starts the
harness as the sandbox workload and enforces the boundary outside the
agent's process.

That boundary is the answer to the accepted risk recorded in
[the security review](security-review.md): `bash` can do anything the user
can, and the in-process approval dialog cannot contain a compromised or
prompt-injected agent. OpenShell can.

The integration is deliberately optional. Nothing changes for users who
run the binary directly.

## What you need

- Linux, macOS on Apple Silicon, or Windows with WSL 2 (experimental)
- Docker, Podman, or a VM runtime for the local gateway
- OpenShell CLI 0.1.x and a selected gateway
  ([install](https://docs.nvidia.com/openshell/latest/about/installation))
- The agent-harness image: a published release tag, or a local build

```bash
# Published on release tags (0.3.40 and later).
docker pull ghcr.io/ba-calderonmorales/agent-harness:0.3.40

# Or build the same image from this checkout.
docker build -t agent-harness:local .
```

The image is Ubuntu 24.04 with `agent-harness`, `bash`, `curl`, `git`,
`openssh-client`, and CA certificates, running as the unprivileged
`sandbox` user with `/workspace` as the workdir. It has no Node or Python
toolchain by design; add what your repositories need to a derived image.

## Quickstart

The examples live in [`examples/openshell/`](../examples/openshell/).
They target OpenRouter because it needs one endpoint and one key. Change
the profile and policy together for another provider.

```bash
# One-time: make the provider available to the gateway.
openshell profile lint -f examples/openshell/provider-openrouter.yaml
openshell profile import -f examples/openshell/provider-openrouter.yaml
OPENROUTER_API_KEY=... openshell provider create \
  --name openrouter --type agent-harness-openrouter --from-existing

# Create the sandbox and upload the repository into /workspace.
# There is no live bind mount in this release, so upload and download are
# explicit steps.
openshell sandbox create \
  --name ah \
  --from ghcr.io/ba-calderonmorales/agent-harness:0.3.40 \
  --provider openrouter \
  --policy examples/openshell/policy-minimal.yaml \
  --upload .:/workspace \
  --detach

# Run the TUI in a separate exec session.
openshell sandbox exec -n ah --tty -- agent-harness

# When the session ends, copy changes back out.
openshell sandbox download ah /workspace ./sandbox-out
```

A non-interactive smoke test that does not need a model:

```bash
openshell sandbox exec -n ah --no-login-shell -- agent-harness --version
```

Use `policy-coding.yaml` instead of the minimal policy when the agent
should clone, fetch, or push over HTTPS. It adds GitHub on 443 for `git`
and its HTTP helper. SSH remotes also need a key inside the sandbox and a
`/home/sandbox/.ssh` entry in the filesystem policy.

## How credentials stay outside the sandbox

The provider profile exposes `OPENROUTER_API_KEY` to the sandbox as a
placeholder and binds the real value to `openrouter.ai` from
`/usr/local/bin/agent-harness`. The harness reads that variable at boot;
OpenShell substitutes the real key only on matching requests. The key is
never readable by the agent, and a request to any other host carries no
credential.

The harness reads these variables from the environment: `AH_API_KEY`,
`AGENT_HARNESS_API_KEY`, `OPENROUTER_API_KEY`, and `NVIDIA_API_KEY`.
If you adapt one of NVIDIA's other
[example profiles](https://github.com/NVIDIA/OpenShell/tree/main/providers)
for Anthropic or OpenAI, change its `env_vars` to `[AH_API_KEY]` and its
`binaries` to `/usr/local/bin/agent-harness`. NVIDIA's copies use
provider-native names such as `ANTHROPIC_API_KEY`, which the harness does
not read at boot.

Running `/login` inside the sandbox also works, but it stores the key in
the sandbox's credential file. The provider profile is the better path.

## What the sandbox changes about the harness

| Area | Inside OpenShell |
|---|---|
| `bash`, `git`, package installs | Confined to allowed paths and destinations; a denied call returns a policy error the model can read |
| File tools | Only paths in `filesystem_policy` are readable/writable; `/workspace` is included automatically |
| `web_fetch` | Still applies its own public-address rule; OpenShell applies the egress policy on top |
| Approval dialog | Unchanged. It is an in-process UX guard; OpenShell is the boundary that survives a compromised agent |
| Sessions and audit | Written under `/home/sandbox/.local/share/agent-harness`, allowed by the example policy. Export to `/workspace` if you need the file outside |
| TUI | Runs through `sandbox exec --tty`; the sandbox's main process is the login shell |

## Limitations

- **No live workspace mount.** `sandbox create` has no bind-mount flag in
  0.1.x. Upload before the session and download after it; long sessions
  can use `sandbox upload` again while running.
- **Platform coverage.** OpenShell requires Docker/Podman/VM and does not
  cover Termux or native Windows without WSL 2. Those users keep the
  direct binary path.
- **Two policy systems.** The harness's permission modes decide which
  tools may run; OpenShell decides what the operating system and network
  permit. They compose, but neither replaces the other.
- **Image updates.** The harness inside the sandbox is the image's
  version. Pull or build a new image to upgrade.
- **OpenShell is 0.x.** Command names and profile schema can change
  between minor releases; this page targets 0.1.5. Treat the policy and
  profile files as reviewable starting points, not frozen defaults.
- **Sentry is not part of this path.** It is a hardware reference design
  for BlueField-4 DPUs and is not downloadable; see the
  [decision record](design/openshell-integration-decision.md).

## Verifying a sandbox

```bash
# The boundary is default-deny: this should be refused.
openshell sandbox exec -n ah --no-login-shell -- curl -sS https://example.com

# The allowed endpoint should work when the provider is attached.
openshell sandbox exec -n ah --no-login-shell -- agent-harness --version

# Read every allow and deny decision.
openshell logs ah --since 10m --source sandbox
```

Policy changes are hot-reloadable for the network section:

```bash
openshell policy update ah \
  --rule-name github_git \
  --binary /usr/bin/git \
  --add-endpoint github.com:443:full:rest:enforce \
  --wait
```

Filesystem, Landlock, and process settings are locked at sandbox creation;
changing them means deleting and recreating the sandbox.

## References

- [OpenShell documentation](https://docs.nvidia.com/openshell/latest/)
- [Policy schema](https://docs.nvidia.com/openshell/latest/how-it-works/policies/schema)
- [Provider profiles](https://docs.nvidia.com/openshell/latest/how-it-works/providers/profiles)
- [Decision record](design/openshell-integration-decision.md)
