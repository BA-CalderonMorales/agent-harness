# Release Workflow Skill

> **Purpose:** Prevent version mismatches and ensure consistent release tagging.
> **Scope:** Version validation, GitHub release checking, automated version bumping.
> **Location:** Stored within agent-harness repo at `.agent-harness/skills/release-workflow/`
> **Note:** No emojis. Use text indicators only.

---

## 1. The Problem

Version mismatches happen when:
- Code version doesn't match the git tag
- Git tag doesn't match the GitHub release
- Multiple sources of truth for version numbers
- Manual steps are forgotten

## The Problem
```
GitHub Release: v0.0.42
Code Version:   "0.0.41"  [MISMATCH!]
Git Tag:        v0.0.42
```

---

## 2. Pre-Release Validation Checklist

### 2.1 Check Current State

```bash
#!/bin/bash
# save as: ~/projects/check-version.sh

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
    exit 1
fi
```

### 2.2 Determine Next Version

```bash
#!/bin/bash
# save as: ~/projects/bump-version.sh

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

# 2. Check current versions
~/projects/check-version.sh

# 3. If mismatch found, fix code version first
# Edit cmd/*/main.go to match GitHub release

# 4. Determine next version
NEW_VERSION=$(~/projects/bump-version.sh v0.0.42 patch)  # or minor/major
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
```

### 3.2 One-Command Release Script

```bash
#!/bin/bash
# save as: ~/projects/release.sh
# Usage: release.sh [patch|minor|major]

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
echo ""
echo "Or run: cd $REPO_DIR && ./scripts/release/publish.sh v$NEW_VERSION"
```

---

## 4. Quick Reference

### Check Versions Anywhere
```bash
cd ~/projects/agent-harness
~/projects/check-version.sh
```

### Create New Release
```bash
cd ~/projects/agent-harness
~/projects/release.sh patch   # or minor, major
```

### Manual Version Fix (Emergency)
```bash
# If version is out of sync:
1. Edit cmd/agent-harness/main.go → update Version string
2. git add -A && git commit -m "fix: align version with release"
3. git push origin develop
```

---

## 5. Version Format

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

## 6. Troubleshooting

| Problem | Solution |
|---------|----------|
| "Code version != Git tag" | Update code version first, then tag |
| "Git tag already exists" | Use different version or delete tag: `git tag -d v0.0.43` |
| "GitHub release already exists" | Cannot overwrite. Use new version. |
| Forgot to bump version | Fix immediately: edit code, commit, force-push (if not main) |

---

> **Last Updated:** 2026-04-04  
> **Purpose:** Never have version mismatches again.
