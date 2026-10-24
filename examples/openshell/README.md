# OpenShell examples

Policy and provider-profile starting points for running the agent-harness
image under [NVIDIA OpenShell](https://github.com/NVIDIA/OpenShell). The
walkthrough lives in [docs/openshell.md](../../docs/openshell.md).

| File | Purpose |
|---|---|
| `policy-minimal.yaml` | OpenRouter inference only; no source hosts or registries |
| `policy-coding.yaml` | The minimal boundary plus GitHub over HTTPS for `git` |
| `provider-openrouter.yaml` | Keeps `OPENROUTER_API_KEY` outside the sandbox and injects it only at `openrouter.ai` |

Treat these as reviewable starting points, not defaults. The policy's
`binaries` lists are the least-privilege control: they must name the real
executable paths in your image, and a rule that lists no binary allows
nothing.

Check the policy and profile before using them:

```bash
openshell profile lint -f examples/openshell/provider-openrouter.yaml
```

`openshell` validates policy YAML when the sandbox is created. Filesystem
rules are static, so a path change means recreating the sandbox; network
rules can be updated on a running sandbox with `openshell policy set`.
