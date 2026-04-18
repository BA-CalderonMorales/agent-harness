#!/bin/bash
# check-remote.sh - ALWAYS check remote version before release
# Usage: ./scripts/release/check-remote.sh
#
# CRITICAL: Uses git ls-remote (not git describe) because git describe
# returns the latest tag reachable from HEAD, which on 'develop' may be
# far behind the actual latest release tag (which is on 'main').

set -e

REPO_DIR="${1:-.}"
cd "$REPO_DIR" || exit 1

echo "=== Remote Version Check (Source of Truth) ==="
echo "Fetching remote tags..."
git fetch --tags origin 2>/dev/null || {
    echo "[!] ERROR: Cannot fetch from origin. Check network and repo access."
    exit 1
}

# Get latest remote tag using ls-remote (sorted by version)
# This is correct regardless of which branch you're on.
REMOTE_TAG=$(git ls-remote --tags origin 2>/dev/null | \
    grep -oE 'refs/tags/v[0-9]+\.[0-9]+\.[0-9]+$' | \
    sort -V | tail -1 | sed 's/refs\/tags\///')

if [ -z "$REMOTE_TAG" ]; then
    echo "[!] No remote version tags found (expected vX.Y.Z format)"
    exit 1
fi

echo "Remote latest: $REMOTE_TAG"

# Cross-check with GitHub releases API
if command -v gh >/dev/null 2>&1; then
    GH_RELEASE=$(gh release list --limit 1 --json tagName -q '.[0].tagName' 2>/dev/null || echo "NONE")
    if [ "$GH_RELEASE" != "NONE" ] && [ "$GH_RELEASE" != "$REMOTE_TAG" ]; then
        echo "[!] WARNING: Remote git tag ($REMOTE_TAG) != GitHub release ($GH_RELEASE)"
        echo "    Using GitHub release as source of truth."
        REMOTE_TAG="$GH_RELEASE"
    fi
fi

# Show local code version for comparison
CODE_VERSION=$(grep -E 'Version\s*=\s*"[^"]+"' cmd/*/main.go 2>/dev/null | head -1 | sed 's/.*"\([^"]*\)".*/\1/')
CODE_V="v${CODE_VERSION}"
echo "Code version:  $CODE_V"

# Validate alignment
if [ "$CODE_V" != "$REMOTE_TAG" ]; then
    echo ""
    echo "[!] MISMATCH: Code ($CODE_V) != Remote ($REMOTE_TAG)"
    echo "    Fix: git fetch --tags origin && git checkout main && git pull"
    exit 1
fi

echo "[OK] All aligned at $REMOTE_TAG"
echo ""

# Show next versions
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
echo "Next versions:"
echo "  patch: $(bash "$SCRIPT_DIR/bump-version.sh" "${REMOTE_TAG#v}" patch)"
echo "  minor: $(bash "$SCRIPT_DIR/bump-version.sh" "${REMOTE_TAG#v}" minor)"
echo "  major: $(bash "$SCRIPT_DIR/bump-version.sh" "${REMOTE_TAG#v}" major)"
