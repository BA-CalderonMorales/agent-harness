# Release Patterns Quick Reference

> One-page pointer for the release checks.
>
> The authoritative runbook is `../release-workflow/SKILL.md`. Where the two
> disagree, that file wins.

---

## The Golden Rules

1. **Remote first** - the remote is the source of truth; anchor on `origin/*`
2. **One change per commit** - never combine a version bump with a feature
3. **Never merge on red** - CI green before every merge
4. **Never end on `main`** - the cycle closes by cutting the next release branch off develop

---

## Version Alignment Check

```bash
CODE=$(grep -E 'Version\s*=\s*"[^"]+"' cmd/*/main.go | sed 's/.*"\([^"]*\)".*/\1/')
# Remote, not git describe: `git describe --tags` returns the newest tag
# reachable from HEAD, and release tags land on main - on develop it
# reports an older release.
GIT=$(git ls-remote --tags origin | grep -oE 'refs/tags/v[0-9]+\.[0-9]+\.[0-9]+$' | sort -V | tail -1 | sed 's|refs/tags/v||')
GH=$(gh release list --limit 1 --json tagName -q '.[0].tagName' | sed 's/^v//')

echo "Code: $CODE | Remote: $GIT | GitHub: $GH"
[ "$CODE" = "$GIT" ] && [ "$GIT" = "$GH" ] && echo "[OK]" || echo "[!] Mismatch"
```

Expect a mismatch between code and remote **after** the version bump commit:
the release is pending, not drifting. This is also why `make release` fails
mid-cycle.

---

## Cross-Repo Sync Checklist

Each repo runs the full remote flow on its own; there is no batch command.

- [ ] agent-harness released
- [ ] lumina-bot released
- [ ] terminal-jarvis released
- [ ] Every repo's develop and main synced from origin
- [ ] Every repo's CD green and release published
- [ ] All versions aligned
- [ ] Next release/X.Y.Z+1 cut in each

---

## Emergency Hotfix

Remote CD is the preferred route even for a hotfix: cut the fix on a
`release/X.Y.Z` branch and let CI build and publish it. There is no
documented local shortcut - a locally built and locally tagged release carries
no pipeline provenance, no pipeline checksums, and no release record. If CD is
genuinely unavailable, say so plainly before reaching for anything else, and
follow the traps section of the runbook.

---

## Where the commands live

| Task | Home |
|------|------|
| The ship sequence | `../release-workflow/SKILL.md` |
| Version alignment, multi-repo sync, anti-patterns | `SKILL.md` in this directory |
| Tag and semver gates | `scripts/check-version-matches.sh`, `.githooks/pre-push` |
| Branch naming gate | `scripts/check-branch-name.sh` |
| Prune merged branches | `scripts/prune-branches.sh --dry-run` first |

`scripts/release.sh` and `scripts/release/publish.sh` are superseded. They push
straight to `develop` and `main`, bypassing the release branch, its naming
gate, PR review and CI. Do not run them.
