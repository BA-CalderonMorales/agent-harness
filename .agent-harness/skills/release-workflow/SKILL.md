---
name: release-workflow
description: Ship agent-harness. A release is a tag on origin/main, cut by make release after CI passes and published by release.yml. Use for "ship it", "cut a release", or "release vX.Y.Z".
---

# Release Workflow

> **Purpose:** ship what is on `main`, at any moment.
> **Trigger:** "ship it", "cut a release", "release vX.Y.Z".

`main` is always releasable, and a release is a tag on `origin/main`. Nothing
in the source holds a version, so there is no bump commit, no release branch,
and no second PR. The user decides when to ship; do not tag without being
asked.

## The sequence

**1. Land the work on `main`.** Each change is a PR from a short-lived branch
into `main`, merged once CI is green. Unfinished work merges behind a feature
flag rather than waiting on a branch.

**2. Preview the release.**

```bash
make release-dry-run
```

It fetches `origin`, finds the last tag reachable from `origin/main`, computes
the next version from the conventional commits since then, and lists them.
Before 1.0.0, a breaking change bumps minor and everything else bumps patch.
Override with `RELEASE_ARGS="--bump minor"` or `RELEASE_ARGS="--version vX.Y.Z"`.

**3. Release.**

```bash
make release
```

It refuses unless the latest CI run on `origin/main` passed, asks once, then
tags `origin/main` and pushes the tag. It never tags the local checkout, so it
works from any branch. Without a terminal, pass `RELEASE_ARGS=--yes`.

**4. Watch CD until the release lands.** Do not call it shipped because the tag
pushed.

```bash
gh run list --workflow=release.yml --limit 3
gh run watch <run_id>
gh release view vX.Y.Z --json assets -q '.assets[].name'
```

Expect six platform archives plus `checksums.txt`. Release notes are generated
from the merged PRs.

**5. Report what landed:** the tag, the release URL, and the PRs it carries.

## Traps

- **Never tag by hand from a local branch.** `make release` targets
  `origin/main`; a hand-made tag on a stale local `main` ships the wrong commit.
- **Never fill a CD gap with a local build.** If `release.yml` fails, fix the
  workflow and re-run it. A locally built release has no CI provenance; if it
  is ever unavoidable, say so plainly.
- **Never edit the user's settings to unblock a release.** Changing
  `~/.config/agent-harness/settings.json` to make a local run work hides a bug
  that a downloader will hit. Fix the code path instead.
- **A failed CI run blocks the release on purpose.** Fix `main`, let CI pass,
  then release. `--skip-ci-check` exists for tests and emergencies only.

## Files

| File | Role |
|---|---|
| `scripts/release/release.sh` | Tags `origin/main` and pushes the tag (`make release`) |
| `scripts/release/next-version.sh` | Computes the next version from conventional commits |
| `.github/workflows/release.yml` | Builds, checksums, and publishes on a tag push |
| `cmd/agent-harness/version.go` | How a binary resolves the version it reports |
| `.githooks/pre-push` | Rejects non-semver version tags |
