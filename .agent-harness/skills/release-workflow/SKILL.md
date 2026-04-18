# Release Workflow Skill

> **Purpose:** Zero-surprise releases with a single command.
> **Scope:** Version validation, automated bumping, merge→main→tag flow.
> **Location:** `.agent-harness/skills/release-workflow/`

---

## 0. The Golden Rule

**Remote is ALWAYS the source of truth.**

**Why `git describe` is dangerous:**

```
$ git checkout develop
$ git describe --tags --abbrev=0
v0.1.5        ← WRONG! Latest is actually v0.1.9 on main

$ git ls-remote --tags origin | sort -V | tail -1
refs/tags/v0.1.9   ← CORRECT! Reads remote directly
```

`git describe` returns the latest tag *reachable from HEAD*. On `develop`,
this lags behind because release tags land on `main`. All scripts here use
`git ls-remote --tags origin` instead.

---

## 1. One-Command Release (New)

```bash
cd ~/projects/agent-harness
./scripts/release.sh patch
```

**What it does:**

1. Validates you're on `develop` with a clean tree
2. Fetches remote, calculates next version from **remote tag** (not local)
3. Bumps `Version` in `cmd/agent-harness/main.go`
4. Prepends entry to `docs/changelog.md`
5. Commits: `chore(release): bump version to vX.Y.Z`
6. Pushes `develop`
7. Checks out `main`, merges `develop`, pushes `main`
8. Creates **annotated tag on main**, pushes tag
9. Polls GitHub for release creation (up to 60s)
10. Returns to `develop`

**Result:** You run one command and end up back on `develop` with everything
published.

---

## 2. Pre-Release Sanity Check

```bash
./scripts/release/check-remote.sh
```

**Output:**

```
=== Remote Version Check (Source of Truth) ===
Fetching remote tags...
Remote latest: v0.1.9
Code version:  v0.1.9
[OK] All aligned at v0.1.9

Next versions:
  patch: 0.1.10
  minor: 0.2.0
  major: 1.0.0
```

Run this before `release.sh` if you want to verify state first.

---

## 3. Manual Step-by-Step (Fallback)

If the one-command script fails or you need granular control:

```bash
# 1. Ensure clean develop
git checkout develop
git pull origin develop

# 2. Check state
./scripts/release/check-remote.sh

# 3. Bump locally
./scripts/release/bump-version.sh patch
# → edits main.go + changelog, commits, creates local tag

# 4. Push develop
git push origin develop

# 5. Merge to main
git checkout main
git pull origin main
git merge develop
git push origin main

# 6. Move tag to main (if bump-version.sh created it on develop)
git tag -d v0.1.10
git tag -a v0.1.10 -m "Release v0.1.10"
git push --delete origin v0.1.10 2>/dev/null || true
git push origin v0.1.10

# 7. Return to develop
git checkout develop
```

---

## 4. Version Format

Semantic versioning:

| Bump | When | Example |
|------|------|---------|
| patch | Bug fixes, tests, DX hardening | v0.1.9 → v0.1.10 |
| minor | New features | v0.1.9 → v0.2.0 |
| major | Breaking changes | v0.1.9 → v1.0.0 |

---

## 5. Troubleshooting

| Problem | Cause | Fix |
|---------|-------|-----|
| "Not on develop branch" | You ran release from main or a feature branch | `git checkout develop` |
| "Working tree is not clean" | Uncommitted changes | `git status`, commit or stash |
| "develop is not in sync" | Local develop behind origin | `git pull origin develop` |
| "Code version != remote tag" | Version bump was skipped or reverted | Run `./scripts/release/check-remote.sh`, fix manually |
| Tag exists but release missing | CI still running | Wait 30-60s, or check `gh run list --workflow=release.yml` |
| `git describe` shows old version | Normal on develop — use `git ls-remote` | This script already does |

---

## 6. Files

| File | Purpose |
|------|---------|
| `scripts/release.sh` | **One-command release** (develop→main→tag→verify) |
| `scripts/release/check-remote.sh` | Sanity check before releasing |
| `scripts/release/bump-version.sh` | Semver bump utility |
| `.github/workflows/release.yml` | CI: builds binaries on tag push |
| `cmd/agent-harness/main.go` | Source of truth for code version |
| `docs/changelog.md` | Release history |

---

## 7. What Changed in This Version of the Skill

**Problem with old workflow:**
- `check-remote.sh` used `git describe --tags` → misleading on develop
- `bump-version.sh` created tag on develop → had to manually merge→main→retag
- Three separate scripts with manual steps between each

**Fixes:**
- `release.sh`: single command does the entire flow
- `check-remote.sh`: uses `git ls-remote --tags origin` (correct on any branch)
- Tag is created **on main** after merge, not on develop
- Auto-polls GitHub for release verification
- Always returns to develop

---

> **Last Updated:** 2026-04-18
> **Changes:** Unified into single `release.sh`, fixed git-describe pitfall, tag-on-main guarantee
