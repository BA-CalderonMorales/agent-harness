# Branches and protection

`main` is the only long-lived branch. Work lands on it through PRs from
short-lived branches, and a release is a tag on `origin/main` (see the
release runbook in `.agent-harness/skills/release-workflow/SKILL.md`).

## What guards `main`

- **CI on every PR to `main`.** `ci.yml` runs the build, quality, security,
  secrets, SBOM, script, and sharded test jobs. A red PR does not merge.
- **The release gate.** Cut release refuses to tag unless the latest CI
  run on `origin/main` passed, so a broken `main` cannot ship.
- **GitHub branch protection on `main`** (Settings → Branches):

  | Setting | Value |
  |---|---|
  | Require a pull request | on, with **0** required approvals |
  | Required status checks | `build`, `test`, `quality`, branch up to date |
  | Conversation resolution | required, so review threads get answered |
  | Force pushes and deletion | blocked |
  | Admin bypass | allowed, for emergencies only |

  A solo maintainer cannot approve their own PR, so a required approval
  only forces an admin bypass on every merge. CI is the gate instead.

## Local hooks

The repository ships hooks in `.githooks/`. Git does not use them until you
opt in once per clone:

```bash
git config core.hooksPath .githooks
```

- `pre-commit` blocks session artifacts and unformatted Go files.
- `commit-msg` rejects subjects that are not conventional commits, which
  the release relies on to pick the next version. CI's `commits` job
  checks every commit in a PR the same way, and the `pr-title` workflow
  checks the PR title, which a squash merge can use as the subject.
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
