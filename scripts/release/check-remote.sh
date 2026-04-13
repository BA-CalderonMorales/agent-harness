#!/bin/bash
# check-remote.sh - ALWAYS check remote version before release
# Usage: ./scripts/release/check-remote.sh
# This prevents the "local tag != remote tag" release issue

set -e

REPO_DIR="${1:-.}"
cd "$REPO_DIR" || exit 1

echo "=== Remote Version Check (Source of Truth) ==="
echo "Fetching remote tags..."
git fetch --tags origin 2>/dev/null || {
    echo "[!] ERROR: Cannot fetch from origin. Check network and repo access."
    exit 1
}

# Get remote tags (sorted by version)
REMOTE_TAG=$(git ls-remote --tags origin 2>/dev/null | \
    grep -o 'refs/tags/v[0-9]\+\.[0-9]\+\.[0-9]\+' | \
    sort -V | tail -1 | sed 's/refs\/tags\///')

if [ -z "$REMOTE_TAG" ]; then
    echo "[!] No remote version tags found (expected vX.Y.Z format)"
    exit 1
fi

echo "Remote latest: $REMOTE_TAG"

# Also check GitHub releases API (different from git tags)
if command -v gh >/dev/null 2>&1; then
    GH_RELEASE=$(gh release list --limit 1 --json tagName -q '.[0].tagName' 2>/dev/null || echo "NONE")
    if [ "$GH_RELEASE" != "NONE" ] && [ "$GH_RELEASE" != "$REMOTE_TAG" ]; then
        echo "[!] WARNING: Remote git tag ($REMOTE_TAG) != GitHub release ($GH_RELEASE)"
        echo "    Using GitHub release as source of truth."
        REMOTE_TAG="$GH_RELEASE"
    fi
fi

# Get local tag for comparison
LOCAL_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "NONE")
echo "Local latest:  $LOCAL_TAG"

# Validate match
if [ "$LOCAL_TAG" != "$REMOTE_TAG" ]; then
    echo ""
    echo "[!] MISMATCH DETECTED!"
    echo "    Local:  $LOCAL_TAG"
    echo "    Remote: $REMOTE_TAG"
    echo ""
    echo "Fix: git fetch --tags origin && git checkout $REMOTE_TAG"
    exit 1
fi

echo "[OK] Local matches remote: $REMOTE_TAG"
echo ""
echo "Next version: $(bash "$(dirname "$0")/bump-version.sh" "${REMOTE_TAG#v}" patch)"
