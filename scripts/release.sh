#!/bin/bash
# release.sh - One-command release: develop → main → tag → verify
# Usage: ./scripts/release.sh [patch|minor|major|x.y.z]
#
# This script automates the entire release flow:
#   1. Validate preconditions (branch, clean tree, remote sync)
#   2. Calculate next version from REMOTE (source of truth)
#   3. Bump version in code, commit on develop
#   4. Push develop
#   5. Merge develop → main, push main
#   6. Tag on main, push tag
#   7. Verify release appears on GitHub
#   8. Return to develop

set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BUMP_TYPE="${1:-patch}"
MAIN_GO="cmd/agent-harness/main.go"
CHANGELOG="docs/changelog.md"

cd "$REPO_DIR"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

info() { echo "→ $1"; }
ok()   { echo "  [OK] $1"; }
warn() { echo "  [!] $1"; }
fail() { echo "  [FAIL] $1"; exit 1; }

# Get latest remote version using git ls-remote (works from any branch)
get_remote_version() {
    git ls-remote --tags origin 2>/dev/null | \
        grep -oE 'refs/tags/v[0-9]+\.[0-9]+\.[0-9]+$' | \
        sort -V | tail -1 | sed 's/refs\/tags\///'
}

# Bump semver: input current version (without v), output new version
bump_version() {
    local ver="${1#v}"
    local type="$2"
    local major minor patch
    major=$(echo "$ver" | cut -d. -f1)
    minor=$(echo "$ver" | cut -d. -f2)
    patch=$(echo "$ver" | cut -d. -f3)

    case "$type" in
        major) major=$((major + 1)); minor=0; patch=0 ;;
        minor) minor=$((minor + 1)); patch=0 ;;
        patch|*) patch=$((patch + 1)) ;;
    esac

    echo "${major}.${minor}.${patch}"
}

# ---------------------------------------------------------------------------
# 0. Pre-flight checks
# ---------------------------------------------------------------------------

echo "=== Agent Harness Release ==="
echo ""

info "Fetching remote..."
git fetch --tags origin || fail "Cannot fetch from origin"
ok "Remote fetched"

info "Checking branch..."
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "develop" ]; then
    fail "Not on develop branch (on: $CURRENT_BRANCH). Run: git checkout develop"
fi
ok "On develop branch"

info "Checking working tree..."
if ! git diff-index --quiet HEAD --; then
    fail "Working tree is not clean. Commit or stash changes first."
fi
ok "Working tree clean"

info "Checking remote sync..."
LOCAL_DEVELOP=$(git rev-parse develop)
REMOTE_DEVELOP=$(git rev-parse origin/develop)
if [ "$LOCAL_DEVELOP" != "$REMOTE_DEVELOP" ]; then
    fail "develop is not in sync with origin/develop. Run: git pull origin develop"
fi
ok "develop in sync with origin"

# ---------------------------------------------------------------------------
# 1. Determine version (remote is source of truth)
# ---------------------------------------------------------------------------

echo ""
info "Determining version..."

REMOTE_TAG=$(get_remote_version)
[ -z "$REMOTE_TAG" ] && fail "No remote version tags found"

CODE_VERSION=$(grep -E 'Version\s*=\s*"[^"]+"' "$MAIN_GO" | head -1 | sed 's/.*"\([^"]*\)".*/\1/')
CODE_V="v${CODE_VERSION}"

echo "  Remote latest: $REMOTE_TAG"
echo "  Code version:  $CODE_V"

# Warn if code version is behind remote (shouldn't happen on clean develop)
if [ "$CODE_V" != "$REMOTE_TAG" ]; then
    warn "Code version ($CODE_V) != remote tag ($REMOTE_TAG)"
    warn "Using remote tag as source of truth for bump calculation"
fi

# Calculate new version from REMOTE tag
if [[ "$BUMP_TYPE" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    NEW_VERSION="$BUMP_TYPE"
else
    NEW_VERSION=$(bump_version "${REMOTE_TAG#v}" "$BUMP_TYPE")
fi
TAG="v${NEW_VERSION}"

echo ""
echo "  Release version: $TAG"
read -p "  Proceed? (y/n) " -n 1 -r
echo
[[ $REPLY =~ ^[Yy]$ ]] || { echo "Aborted."; exit 1; }

# ---------------------------------------------------------------------------
# 2. Bump version on develop
# ---------------------------------------------------------------------------

echo ""
info "Bumping version to $NEW_VERSION..."

sed -i "s/Version\s*=\s*\"[^\"]*\"/Version   = \"${NEW_VERSION}\"/" "$MAIN_GO"
ok "Updated $MAIN_GO"

# Update changelog
DATE=$(date +%Y-%m-%d)
TMP=$(mktemp)
{
    echo "# Changelog"
    echo ""
    echo "## [$NEW_VERSION] - $DATE"
    echo ""
    echo "### Changed"
    echo "- Version bump to $TAG"
    echo ""
    tail -n +3 "$CHANGELOG"
} > "$TMP"
mv "$TMP" "$CHANGELOG"
ok "Updated $CHANGELOG"

# Commit
git add "$MAIN_GO" "$CHANGELOG"
git commit -m "chore(release): bump version to $TAG"
ok "Committed version bump"

# ---------------------------------------------------------------------------
# 3. Push develop
# ---------------------------------------------------------------------------

echo ""
info "Pushing develop..."
git push origin develop
ok "develop pushed"

# ---------------------------------------------------------------------------
# 4. Merge develop → main
# ---------------------------------------------------------------------------

echo ""
info "Merging develop → main..."
git checkout main
git pull origin main
git merge develop --no-edit
git push origin main
ok "main merged and pushed"

# ---------------------------------------------------------------------------
# 5. Tag on main
# ---------------------------------------------------------------------------

echo ""
info "Creating tag $TAG on main..."
git tag -a "$TAG" -m "Release $TAG"
git push origin "$TAG"
ok "Tag $TAG pushed"

# ---------------------------------------------------------------------------
# 6. Verify release
# ---------------------------------------------------------------------------

echo ""
info "Verifying release..."

# Wait for GitHub Actions to create the release
for i in {1..30}; do
    if gh release view "$TAG" >/dev/null 2>&1; then
        ok "Release $TAG published on GitHub"
        gh release view "$TAG" --json url -q '.url'
        break
    fi
    sleep 2
    echo "  ... waiting for CI ($i/30)"
done

if ! gh release view "$TAG" >/dev/null 2>&1; then
    warn "Release $TAG not yet visible on GitHub"
    warn "Check CI status: gh run list --workflow=release.yml"
fi

# ---------------------------------------------------------------------------
# 7. Return to develop
# ---------------------------------------------------------------------------

echo ""
info "Returning to develop..."
git checkout develop
ok "Back on develop"

echo ""
echo "=== Release $TAG Complete ==="
echo "  develop: $(git rev-parse --short develop)"
echo "  main:    $(git rev-parse --short main)"
echo "  tag:     $TAG"
