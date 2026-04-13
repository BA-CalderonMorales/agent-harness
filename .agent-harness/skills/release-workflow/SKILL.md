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

### 3.1 Step-by-Step Release Process

```bash
# BEFORE any release, run these commands:

# 1. Ensure you're on develop branch
git checkout develop
git pull origin develop

# 2. Check current versions (ALWAYS check remote first!)
./scripts/release/check-remote.sh

# 3. If mismatch found, fix first
git fetch --tags origin
git checkout develop  # ensure on correct branch

# 4. Determine next version
NEW_VERSION=$(./scripts/release/bump-version.sh v0.1.1 patch)  # uses remote as source
echo "Next version: v$NEW_VERSION"

# 5. Update code version
# Edit cmd/agent-harness/main.go:
# Version = "0.0.43"

# 6. Update changelog
cat >> docs/changelog.md << EOF
## [0.0.43] - $(date +%Y-%m-%d)

### Fixed
- Version alignment issue

EOF

# 7. Commit version bump
git add -A
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

echo "✓ Release complete. Now on develop branch - ready for shipping."
```

### 3.2 One-Command Release Script

Uses `scripts/release/release.sh`:

```bash
#!/bin/bash
# Usage: ./scripts/release/release.sh [patch|minor|major]
# CRITICAL: Fetches remote tags FIRST (prevents mismatch)

set -e

REPO_DIR="${REPO_DIR:-~/projects/agent-harness}"
BUMP_TYPE="${1:-patch}"

cd "$REPO_DIR"

echo "=== Starting Release Process ==="
echo "Repository: $REPO_DIR"
echo "Bump type: $BUMP_TYPE"
echo ""

# 1. Check current state
echo "→ Checking current versions..."
CODE_VERSION=$(grep -E 'Version\s*=\s*"[^"]+"' cmd/*/main.go 2>/dev/null | head -1 | sed 's/.*"\([^"]*\)".*/\1/')
GIT_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "NONE")
GH_RELEASE=$(gh release list --limit 1 --json tagName -q '.[0].tagName' 2>/dev/null || echo "NONE")

echo "  Code:   $CODE_VERSION"
echo "  Git:    $GIT_TAG"
echo "  GitHub: $GH_RELEASE"

# 2. Validate we're starting from aligned state
CODE_V="v$CODE_VERSION"
if [ "$CODE_V" != "$GH_RELEASE" ] && [ "$CODE_VERSION" != "$GH_RELEASE" ]; then
    echo ""
    echo "! WARNING: Starting from mismatched state!"
    echo "  Code version does not match GitHub release."
    echo "  Fix this first before creating new release."
    exit 1
fi

# 3. Calculate new version
echo ""
echo "→ Calculating new version..."
LATEST="${GH_RELEASE#v}"
NEW_VERSION=$(bash ~/projects/bump-version.sh "$LATEST" "$BUMP_TYPE")
echo "  New version: v$NEW_VERSION"

# 4. Update code
echo ""
echo "→ Updating code version..."
sed -i "s/Version\s*=\s*\"[^\"]*\"/Version   = \"$NEW_VERSION\"/" cmd/*/main.go

# 5. Update changelog if exists
if [ -f docs/changelog.md ]; then
    echo "→ Updating changelog..."
    TODAY=$(date +%Y-%m-%d)
    # Prepend to changelog
    TEMP=$(mktemp)
    cat > "$TEMP" << EOF
# Changelog

## [$NEW_VERSION] - $TODAY

### Changed
- Version bump to v$NEW_VERSION

EOF
    tail -n +3 docs/changelog.md >> "$TEMP"
    mv "$TEMP" docs/changelog.md
fi

# 6. Commit
echo ""
echo "→ Committing changes..."
git add -A
git commit -m "chore(release): bump version to v$NEW_VERSION"

# 7. Push
echo ""
echo "→ Pushing to develop..."
git push origin develop

echo ""
echo "=== Release Preparation Complete ==="
echo ""
echo "Next steps:"
echo "  1. Merge develop to main:"
echo "     git checkout main && git merge develop"
echo "  2. Push main: git push origin main"
echo "  3. Create tag: git tag -a v$NEW_VERSION -m 'Release v$NEW_VERSION'"
echo "  4. Push tag: git push origin v$NEW_VERSION"
echo "  5. Return to develop: git checkout develop"
echo ""
echo "Or run: cd $REPO_DIR && ./scripts/release/publish.sh v$NEW_VERSION"
```

---

## 4. Quick Reference

### Check Versions (Remote-First)
```bash
cd ~/projects/agent-harness
make check-remote
# or: ./scripts/release/check-remote.sh
```

### Create New Release (Safe)
```bash
cd ~/projects/agent-harness
make release    # checks remote, then builds
# or: ./scripts/release/release.sh patch
```

### Manual Version Fix (Emergency)
```bash
# If version is out of sync:
1. Fetch remote: git fetch --tags origin
2. Check mismatch: ./scripts/release/check-remote.sh
3. Checkout correct tag: git checkout v0.1.1
4. Fix code: Edit cmd/agent-harness/main.go → update Version string
5. Commit: git add -A && git commit -m "fix: align version with release"
6. Push: git push origin develop
```

### Makefile Targets
```bash
make check-remote   # Validate local == remote (safe)
make release        # check-remote + build (creates release binary)
make build          # Build only (uses local git state)
```

---

## 5. Files in This Workflow

| File | Purpose |
|------|---------|
| `scripts/release/check-remote.sh` | Fetches remote, validates local == remote |
| `scripts/release/release.sh` | Full release process (remote-first) |
| `scripts/release/bump-version.sh` | Calculates next semver version |
| `scripts/release/check-version.sh` | Local validation only (legacy) |
| `Makefile` | `make release` = check-remote + build |
| `.githooks/pre-push` | Optional: prevents bad tag pushes |

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

> **Last Updated:** 2026-04-12  
> **Changes:** Added remote-first validation (check-remote.sh, Makefile `release` target)  
> **Purpose:** Never have version mismatches again.
