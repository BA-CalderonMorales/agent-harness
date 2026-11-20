---
name: release-workflow
description: Ship agent-harness. A release is a tag on origin/main, cut by the Cut release GitHub Actions workflow after CI passes, which then starts release.yml and container.yml on the tag. Use for "ship it", "cut a release", or "release vX.Y.Z".
---

# Release Workflow

> **Purpose:** ship what is on `main`, at any moment, from GitHub Actions.
> **Trigger:** "ship it", "cut a release", "release vX.Y.Z".

`main` is always releasable, and a release is a tag on `origin/main`. Nothing
in the source holds a version, so there is no bump commit, no release branch,
and no second PR. Releases are cut in GitHub Actions, not on a laptop. The user
decides when to ship; do not start a release without being asked.

## The sequence

**1. Land the work on `main`.** Each change is a PR from a short-lived branch
into `main`, merged once CI is green. Unfinished work merges behind a feature
flag rather than waiting on a branch.

**2. Preview the release.**

```bash
gh workflow run cut-release.yml -f dry_run=true
```

The run's summary shows the next version and the commits it carries. It finds
the last stable tag reachable from `origin/main` and computes the next version
from the conventional commits since then. Before 1.0.0, a breaking change bumps
minor and everything else bumps patch. `make release-dry-run` shows the same
locally without changing anything.

**3. Release.** From the Actions tab (Cut release → Run workflow, on `main`), or:

```bash
gh workflow run cut-release.yml
gh workflow run cut-release.yml -f bump=minor
gh workflow run cut-release.yml -f version=v1.0.0-rc.1
```

It refuses unless the latest CI run on `origin/main` passed and, for an
automatic bump, every commit since the last tag is a conventional commit. It
then tags `origin/main` as the GitHub Actions bot and starts `release.yml` and
`container.yml` on the tag: a tag pushed with the workflow token triggers
nothing by itself.

**4. Watch CD until the release lands.** Do not call it shipped because the tag
pushed.

```bash
gh run list --workflow=release.yml --limit 3
gh run watch <run_id>
gh release view vX.Y.Z --json assets -q '.assets[].name'
```

Expect six platform archives plus `checksums.txt`, and a container image tagged
with the version and `latest`. Release notes are generated from the merged PRs.

**5. Report what landed:** the tag, the release URL, and the PRs it carries.

## Traps

- **Never tag by hand.** Cut release targets `origin/main`; a hand-made tag on
  a stale local `main` ships the wrong commit. `make release` runs the same
  script locally and exists only as a fallback when Actions is unavailable.
- **Never fill a CD gap with a local build.** If `release.yml` fails, fix the
  workflow and re-run it on the tag (`gh workflow run release.yml --ref vX.Y.Z`).
  A locally built release has no CI provenance; if it is ever unavoidable, say
  so plainly.
- **Never edit the user's settings to unblock a release.** Changing
  `~/.config/agent-harness/settings.json` to make a local run work hides a bug
  that a downloader will hit. Fix the code path instead.
- **A failed CI run blocks the release on purpose.** Fix `main`, let CI pass,
  then release. `--skip-ci-check` exists for tests and emergencies only.

## Files

| File | Role |
|---|---|
| `.github/workflows/cut-release.yml` | Runs the release from Actions, then starts publishing on the tag |
| `scripts/release/release.sh` | Tags `origin/main` and pushes the tag; Cut release runs it |
| `scripts/release/next-version.sh` | Computes the next version from conventional commits |
| `.github/workflows/release.yml` | Builds, checksums, and publishes the binaries for a tag |
| `.github/workflows/container.yml` | Builds and publishes the container image for a tag |
| `cmd/agent-harness/version.go` | How a binary resolves the version it reports |
