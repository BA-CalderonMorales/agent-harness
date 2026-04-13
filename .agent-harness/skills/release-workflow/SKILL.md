# Release Workflow Skill

> **Purpose:** Prevent version mismatches and ensure consistent release tagging.
> **Scope:** Version validation, GitHub release checking, automated version bumping.
> **Location:** Stored within agent-harness repo at `.agent-harness/skills/release-workflow/`
> **Note:** No emojis. Use text indicators only.

---

## 0. The Golden Rule (CRITICAL)

**Remote tag is ALWAYS the source of truth.**

```
Local:  v0.1.0 (old)
Remote: v0.1.1 (current)

WRONG:  Use local → calculates v0.1.1 (already exists!)
RIGHT:  Fetch remote → calculates v0.1.2 (correct!)
```

All scripts in `scripts/release/` fetch remote first to prevent mismatches.

---

## 1. The Problem

Version mismatches happen when:
- Code version doesn't match the git tag
- Git tag doesn't match the GitHub release
- Multiple sources of truth for version numbers
- Manual steps are forgotten
- **Local tags are stale (don't fetch remote)**

### Example Mismatch (What We Fixed)
```
GitHub Release: v0.1.1
Git Tag Local:  v0.1.0  (stale!)
Code Version:   "0.0.55" (fallback)

User runs release.sh → calculates v0.1.1 → ERROR: already exists!
```

### After Fix
```
$ make release
=== Remote Version Check ===
Fetching remote tags...
Remote latest: v0.1.1
[OK] Local matches remote: v0.1.1
→ New version: v0.1.2
```

---

## 2. Pre-Release Validation Checklist

### 2.1 Check Current State (Remote-First)

Uses `scripts/release/check-remote.sh`:

```bash
#!/bin/bash
# ALWAYS fetches remote tags first (prevents version mismatch)

REPO_DIR="${1:-.}"
cd "$REPO_DIR" || exit 1

echo "=== Version Status Check ==="
echo ""

# 1. Get code version from main.go (Go projects)
CODE_VERSION=$(grep -E 'Version\s*=\s*"[^"]+"' cmd/*/main.go 2>/dev/null | head -1 | sed 's/.*"\([^"]*\)".*/\1/')
if [ -z "$CODE_VERSION" ]; then
    CODE_VERSION="NOT FOUND"
fi
echo "Code Version:   $CODE_VERSION"

# 2. Get latest git tag
GIT_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "NONE")
echo "Latest Git Tag: $GIT_TAG"

# 3. Get latest GitHub release
GH_RELEASE=$(gh release list --limit 1 --json tagName -q '.[0].tagName' 2>/dev/null || echo "NONE")
echo "GitHub Release: $GH_RELEASE"

echo ""
echo "=== Validation ==="

# Check if code version matches git tag (strip 'v' prefix for comparison)
CODE_V="v$CODE_VERSION"
if [ "$CODE_V" != "$GIT_TAG" ] && [ "$CODE_VERSION" != "$GIT_TAG" ]; then
    echo "! WARNING: Code version ($CODE_VERSION) != Git tag ($GIT_TAG)"
    MISMATCH=1
fi

# Check if git tag matches GitHub release
if [ "$GIT_TAG" != "$GH_RELEASE" ]; then
    echo "! WARNING: Git tag ($GIT_TAG) != GitHub release ($GH_RELEASE)"
    MISMATCH=1
fi

if [ -z "$MISMATCH" ]; then
    echo "[OK] All versions aligned"
    exit 0
else
    echo ""
    echo "[ERROR] Version mismatch detected!"
    echo "Fix: git fetch --tags origin && git checkout $REMOTE_TAG"
    exit 1
fi
```

**Quick check:**
```bash
cd ~/projects/agent-harness
make check-remote
# or: ./scripts/release/check-remote.sh
```

### 2.2 Determine Next Version

Uses `scripts/release/bump-version.sh`:

```bash
#!/bin/bash
# Calculates next version from current

CURRENT_VERSION="${1:-}"
BUMP_TYPE="${2:-patch}"  # patch, minor, major

if [ -z "$CURRENT_VERSION" ]; then
    echo "Usage: bump-version.sh <current_version> [patch|minor|major]"
    exit 1
fi

# Strip 'v' prefix if present
VERSION="${CURRENT_VERSION#v}"

# Split into components
MAJOR=$(echo "$VERSION" | cut -d. -f1)
MINOR=$(echo "$VERSION" | cut -d. -f2)
PATCH=$(echo "$VERSION" | cut -d. -f3)

# Bump accordingly
case "$BUMP_TYPE" in
    major)
        MAJOR=$((MAJOR + 1))
        MINOR=0
        PATCH=0
        ;;
    minor)
        MINOR=$((MINOR + 1))
        PATCH=0
        ;;
    patch|*)
        PATCH=$((PATCH + 1))
        ;;
esac

echo "$MAJOR.$MINOR.$PATCH"
```

---

## 3. Release Workflow

### 3.1 Automated Bump (GitHub Actions)

The preferred method: trigger `.github/workflows/bump-version.yml` from the GitHub UI.

**What it does:**
1. Determines new version from input (patch/minor/major or explicit x.y.z)
2. Updates `Version` in `cmd/agent-harness/main.go`
3. Prepends entry to `docs/changelog.md`
4. Commits with `chore(release): bump version to vX.Y.Z`
5. Pushes commit and creates git tag
6. Existing `release.yml` triggers automatically on the new tag

**How to run:**
```bash
# Via CLI (requires gh CLI)
gh workflow run bump-version.yml -f bump=patch

# Or open GitHub UI:
# Actions → Bump Version → Run workflow → choose patch/minor/major
```

### 3.2 Local Bump Script (Fallback)

Uses `scripts/bump-version.sh` when you need to bump locally:

```bash
./scripts/bump-version.sh patch   # or minor, major, or explicit 0.2.0
# Creates commit and tag locally
# Then run:
git push && git push origin v0.1.5
```

### 3.3 Manual Step-by-Step Release Process

```bash
# BEFORE any release, run these commands:

# 1. Ensure you're on develop branch
git checkout develop
git pull origin develop

# 2. Check current versions (ALWAYS check remote first!)
git fetch --tags origin
CODE_VERSION=$(grep -E 'Version\s*=\s*"[^"]+"' cmd/*/main.go 2>/dev/null | head -1 | sed 's/.*"\([^"]*\)".*/\1/')
GIT_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "NONE")
GH_RELEASE=$(gh release list --limit 1 --json tagName -q '.[0].tagName' 2>/dev/null || echo "NONE")
echo "Code: $CODE_VERSION  Git: $GIT_TAG  GitHub: $GH_RELEASE"

# 3. If mismatch found, fix first
git checkout develop

# 4. Determine next version
NEW_VERSION=$(./scripts/bump-version.sh "$CODE_VERSION" patch)
echo "Next version: v$NEW_VERSION"

# 5. Update code version
sed -i 's/Version   = ".*"/Version   = "'$NEW_VERSION'"/' cmd/agent-harness/main.go

# 6. Update changelog
DATE=$(date +%Y-%m-%d)
TMP=$(mktemp)
{
    echo "# Changelog"
    echo ""
    echo "## [$NEW_VERSION] - $DATE"
    echo ""
    echo "### Changed"
    echo "- Version bump to v$NEW_VERSION"
    echo ""
    tail -n +3 docs/changelog.md
} > "$TMP"
mv "$TMP" docs/changelog.md

# 7. Commit version bump
git add cmd/agent-harness/main.go docs/changelog.md
git commit -m "chore(release): bump version to v$NEW_VERSION"

# 8. Push to develop
git push origin develop

# 9. Merge to main (follow your branch strategy)
git checkout main
git merge develop  # or create PR

# 10. Create and push tag
git tag -a "v$NEW_VERSION" -m "Release v$NEW_VERSION"
git push origin "v$NEW_VERSION"

# 11. Verify release was created
git fetch --tags
gh release list --limit 5

# 12. CRITICAL: Return to develop branch for shipping mode
git checkout develop

echo "[OK] Release complete. Now on develop branch - ready for shipping."
```

---

## 4. Quick Reference

### Automated Bump (Preferred)
```bash
gh workflow run bump-version.yml -f bump=patch
```

### Local Bump (Fallback)
```bash
cd ~/projects/agent-harness
./scripts/bump-version.sh patch
git push && git push origin v0.1.5
```

### Check Versions (Remote-First)
```bash
cd ~/projects/agent-harness
git fetch --tags origin
CODE_VERSION=$(grep 'Version\s*=' cmd/agent-harness/main.go | sed 's/.*"\(.*\)".*/\1/')
GIT_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "NONE")
GH_RELEASE=$(gh release list --limit 1 --json tagName -q '.[0].tagName' 2>/dev/null || echo "NONE")
echo "Code: $CODE_VERSION  Git: $GIT_TAG  GitHub: $GH_RELEASE"
```

### Manual Version Fix (Emergency)
```bash
# If version is out of sync:
1. Fetch remote: git fetch --tags origin
2. Check mismatch: compare CODE_VERSION, GIT_TAG, GH_RELEASE
3. Checkout correct tag: git checkout v0.1.1
4. Fix code: Edit cmd/agent-harness/main.go → update Version string
5. Commit: git add -A && git commit -m "fix: align version with release"
6. Push: git push origin develop
```

---

## 5. Files in This Workflow

| File | Purpose |
|------|---------|
| `.github/workflows/bump-version.yml` | GitHub Actions workflow for automated version bump |
| `.github/workflows/release.yml` | Builds and publishes binaries on tag push |
| `scripts/bump-version.sh` | Local semver bump script |
| `cmd/agent-harness/main.go` | Source of truth for code version (`Version` var) |
| `docs/changelog.md` | Human-readable release history |

---

## 6. Version Format

We follow **Semantic Versioning** (semver):
```
vMAJOR.MINOR.PATCH

Examples:
  v0.0.42  → v0.0.43 (patch - bug fixes)
  v0.0.42  → v0.1.0  (minor - new features)
  v0.0.42  → v1.0.0  (major - breaking changes)
```

**When to bump:**
| Type | When | Example |
|------|------|---------|
| Patch | Bug fixes, docs | v0.0.42 → v0.0.43 |
| Minor | New features, enhancements | v0.0.42 → v0.1.0 |
| Major | Breaking API changes | v0.0.42 → v1.0.0 |

---

## 7. Troubleshooting

| Problem | Solution |
|---------|----------|
| "Code version != Git tag" | Update code version first, then tag |
| "Git tag already exists" | Use different version or delete tag: `git tag -d v0.0.43` |
| "GitHub release already exists" | Cannot overwrite. Use new version. |
| Forgot to bump version | Fix immediately: edit code, commit, force-push (if not main) |

---

## 8. Shipping Mode Protocol

> **Rule:** Always end a release back on the `develop` branch.

After pushing the tag, immediately switch back to develop:

```bash
git checkout develop
```

This ensures:
- The next feature work starts from the right branch
- No accidental commits to main
- Clear separation between release and development workflows

---

> **Last Updated:** 2026-04-13  
> **Changes:** Added GitHub Actions bump-version workflow and local bump-version.sh script  
> **Purpose:** Never have version mismatches again.
