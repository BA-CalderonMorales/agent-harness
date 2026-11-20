# Branches and protection

`main` is the only long-lived branch. Work lands on it through PRs from
short-lived branches, and a release is a tag on `origin/main` (see the
release runbook in `.agent-harness/skills/release-workflow/SKILL.md`).

## What guards `main`

- **CI on every PR to `main`.** `ci.yml` runs the build, quality, security,
  secrets, SBOM, script, and sharded test jobs. A red PR does not merge.
- **The release gate.** `make release` refuses to tag unless the latest CI
  run on `origin/main` passed, so a broken `main` cannot ship.
- **GitHub branch protection is currently off.** Turn it on in
  Settings → Branches if you want GitHub to enforce the PR and CI rules.
  A solo maintainer cannot approve their own PR, so do not require an
  approving review; require the `build`, `test`, and `quality` checks
  instead, and restrict deletion of `main`.

## Local hooks

The repository ships hooks in `.githooks/`. Git does not use them until you
opt in once per clone:

```bash
git config core.hooksPath .githooks
```

- `pre-commit` blocks session artifacts and unformatted Go files.
- `pre-push` rejects version tags that are not semver.

## Pruning merged branches

`scripts/prune-branches.sh` deletes local and remote branches already merged
into `main`. It never deletes `main` or an unmerged branch. Preview first:

```bash
./scripts/prune-branches.sh --dry-run
./scripts/prune-branches.sh
```

To prune remote-tracking refs on every fetch:

```bash
git config --local fetch.prune true
```
