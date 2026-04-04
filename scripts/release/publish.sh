#!/bin/bash
# publish.sh - Publish a release from main branch
# Usage: ./scripts/release/publish.sh <version>

set -e

REPO_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
VERSION="${1:-}"

if [ -z "$VERSION" ]; then
    echo "Usage: publish.sh <version>"
    echo "Example: publish.sh v0.0.46"
    exit 1
fi

# Strip 'v' prefix for consistency
VERSION="${VERSION#v}"
TAG="v$VERSION"

cd "$REPO_DIR"

echo "=== Agent Harness Release Publish ==="
echo "Version: $TAG"
echo ""

# 1. Ensure we're on main
echo "→ Checking branch..."
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "main" ]; then
    echo "! Not on main branch (currently on: $CURRENT_BRANCH)"
    read -p "Switch to main and pull latest? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        git checkout main
        git pull origin main
    else
        echo "Aborted. Please switch to main branch first."
        exit 1
    fi
fi

# 2. Verify version in code matches
echo ""
echo "→ Verifying version in code..."
CODE_VERSION=$(grep -E 'Version\s*=\s*"[^"]+"' cmd/*/main.go 2>/dev/null | head -1 | sed 's/.*"\([^"]*\)".*/\1/')
if [ "$CODE_VERSION" != "$VERSION" ]; then
    echo "! WARNING: Code version ($CODE_VERSION) does not match requested version ($VERSION)"
    read -p "Continue anyway? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted."
        exit 1
    fi
fi

echo "  Code version: $CODE_VERSION ✓"

# 3. Check for uncommitted changes
echo ""
echo "→ Checking for uncommitted changes..."
if ! git diff-index --quiet HEAD --; then
    echo "! WARNING: Uncommitted changes detected"
    git status --short
    echo ""
    read -p "Continue anyway? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted. Please commit or stash changes first."
        exit 1
    fi
else
    echo "  Working directory clean ✓"
fi

# 4. Build to verify
echo ""
echo "→ Verifying build..."
if go build -o /tmp/agent-harness-verify ./cmd/agent-harness; then
    echo "  Build successful ✓"
    /tmp/agent-harness-verify --version
    rm -f /tmp/agent-harness-verify
else
    echo "! Build failed"
    exit 1
fi

# 5. Create and push tag
echo ""
echo "→ Creating tag $TAG..."
if git rev-parse "$TAG" >/dev/null 2>&1; then
    echo "! Tag $TAG already exists"
    read -p "Delete and recreate? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        git tag -d "$TAG"
        git push origin --delete "$TAG" 2>/dev/null || true
    else
        echo "Aborted."
        exit 1
    fi
fi

git tag -a "$TAG" -m "Release $TAG"
echo "  Tag created ✓"

echo ""
echo "→ Pushing tag to origin..."
git push origin "$TAG"
echo "  Tag pushed ✓"

# 6. Verify CI triggered
echo ""
echo "=== Release Published ==="
echo ""
echo "Tag $TAG has been pushed. CI/CD will now:"
echo "  - Build binaries for Linux and macOS"
echo "  - Create GitHub release with artifacts"
echo "  - Publish to relevant package managers"
echo ""
echo "Monitor progress:"
echo "  gh run watch"
echo ""
echo "After CI completes, verify the release:"
echo "  gh release view $TAG"
