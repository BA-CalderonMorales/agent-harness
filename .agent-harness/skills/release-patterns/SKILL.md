---
name: release-patterns
description: Version alignment checks, multi-repo release sync, and the release anti-patterns that have actually bitten agent-harness. The ship sequence itself lives in the release-workflow skill; read that first.
---

# Release Patterns

> **Purpose:** the checks that surround a release — version alignment, multi-repo
> sync, and the anti-patterns this repo has paid for.
> **Not the sequence.** `../release-workflow/SKILL.md` is the authoritative
> end-to-end runbook. Where the two disagree, it wins.

---

## Precedence

Remote first, always. The remote is the only source of truth: PRs carry the
changes, remote CI is the gate, remote CD builds and publishes. Anchor on
`origin/*`, never on the working tree. A stale local `main` read 39 commits
behind `origin/main` during the 0.3.37 cycle — enough to compare branches
backwards or tag the wrong commit.

Never push directly to `develop` or `main`. Both go through a PR, and the
`release/X.Y.Z` branch name is a gate, not a preference.

---

## Pattern 1: Version alignment check

Three sources have to agree: the code constant, the newest remote tag, and the
newest GitHub release.

```bash
CODE=$(grep -E 'Version\s*=\s*"[^"]+"' cmd/*/main.go | sed 's/.*"\([^"]*\)".*/\1/')

# Remote, not `git describe --tags`: that returns the newest tag reachable
# from HEAD, and release tags land on main — on develop it reports an older
# release. This is the same source scripts/release/check-remote.sh reads.
GIT=$(git ls-remote --tags origin \
  | grep -oE 'refs/tags/v[0-9]+\.[0-9]+\.[0-9]+$' \
  | sort -V | tail -1 | sed 's|refs/tags/v||')

GH=$(gh release list --limit 1 --json tagName -q '.[0].tagName' | sed 's/^v//')

echo "Code: $CODE | Remote: $GIT | GitHub: $GH"
[ "$CODE" = "$GIT" ] && [ "$GIT" = "$GH" ] && echo "[OK]" || echo "[MISMATCH]"
```

**These agree only *before* the version bump.** Once the release branch carries
`chore(release): bump version to vX.Y.Z`, the code is ahead of the newest
remote tag by design — that is the pending release, not a drift. A mismatch
mid-cycle is expected; `make release` fails for the same reason, because it
wraps this check.

---

## Pattern 2: Multi-repo sync

Each repository runs the `../release-workflow/SKILL.md` sequence independently —
its own release branch, its own PRs, its own tag and CD run. There is no
cross-repo batch command; `scripts/release.sh` is superseded and must not be
used (see the traps in the runbook).

Release in dependency order — `agent-harness` first, then anything that depends on
it — and do not start the next repo until the previous one's CD has published,
so a downstream repo never pins a version that does not exist yet. There is no
loop worth scripting here: each repo's sequence has merge points that need a
human decision, so run them one at a time from the runbook.

Confirm alignment across all of them when the stack is done:

- [ ] Every repo's `develop` and `main` merged and synced
- [ ] Every repo tagged, CD green, release published
- [ ] Version alignment check clean in each
- [ ] Next `release/X.Y.Z+1` branch cut in each

---

## Pattern 3: Post-release verification

Do not call a release shipped because the tag pushed. Confirm the artifacts
exist.

```bash
gh run list --workflow=release.yml --limit 3
gh run watch <run_id>
gh release view vX.Y.Z
gh release view vX.Y.Z --json assets -q '.assets[].name'
```

Expect six platform binaries plus `checksums.txt`. All of it is built by CD on
the remote — never reproduce a build locally to fill a gap in it.

---

## Anti-patterns

### Combined version and feature commit

```
git commit -m "feat: new feature + version bump"   # one change per commit
```

### Tag before CI passes

```
git push origin develop
git tag vX.Y.Z        # CI has not passed, and develop is not main yet
```

### Tagging a local `main`

```
git checkout main && git tag vX.Y.Z    # local main drifts stale
git rev-parse origin/main              # tag this commit instead
```

### Direct push that bypasses the release branch

```
git checkout main && git merge develop && git push origin main   # skips the PR
```

This is what the `release/X.Y.Z` naming gate, the PR checks and the tag gate
exist to prevent. A merged release branch carries review and CI provenance; a
direct push carries neither.

### Using `git describe` to find the latest release

Covered in Pattern 1 — it reports the newest tag reachable from `HEAD`, which
on `develop` is an older release.

### Light tags

```
git tag -a vX.Y.Z -m "Release vX.Y.Z"   # annotated, not a bare tag
```

### A local bump and locally pushed tag that bypasses remote CD

A last resort, not a route. The published artifacts then carry no pipeline
provenance, no pipeline checksums and no release record to audit. Reach for it
only when CD is genuinely unavailable, and say so plainly rather than quietly.

### Editing the user's vendored settings to unblock a release

Changing `~/.config/agent-harness/settings.json` to make a local run work hides
a bug that a downloader will hit. Fix the code path instead.

---

## Release checklist

```markdown
## Release vX.Y.Z

- [ ] Dogfooded; the changes are exercised, not just compiled
- [ ] Version bump committed on release/X.Y.Z with the changelog section
- [ ] PR release/X.Y.Z -> develop opened, CI green
- [ ] Copilot review addressed per comment, threads replied to then resolved
- [ ] Merged to develop
- [ ] PR develop -> main opened, CI green, merged
- [ ] Annotated tag pushed from origin/main
- [ ] CD green; six binaries plus checksums.txt published
- [ ] Local develop and main synced from origin
- [ ] Merged branches pruned (--dry-run first)
- [ ] Next release/X.Y.Z+1 cut off develop
```

---

> **Remember:** remote first, one change per commit, and never merge on red.
