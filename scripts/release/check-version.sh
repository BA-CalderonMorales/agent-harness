#!/bin/bash
# check-version.sh - Validate version alignment across code, git, and GitHub
# Usage: ./scripts/release/check-version.sh [repo_dir]

REPO_DIR="${1:-.}"
cd "$REPO_DIR" || exit 1

echo "=== Version Status Check ==="
echo "Repository: $(pwd)"
echo ""

# 1. Get code version from main.go
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

MISMATCH=0

# Check if code version matches git tag
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

if [ $MISMATCH -eq 0 ]; then
    echo "[OK] All versions aligned"
    exit 0
else
    echo ""
    echo "[ERROR] Version mismatch detected!"
    echo ""
    echo "Fix: Update code version to match GitHub release, then create new release."
    exit 1
fi
