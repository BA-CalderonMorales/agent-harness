#!/bin/bash
# release.sh - One-command release workflow with version validation
# Usage: ./scripts/release/release.sh [patch|minor|major]

set -e

REPO_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
BUMP_TYPE="${1:-patch}"

cd "$REPO_DIR"

echo "=== Agent Harness Release Process ==="
echo "Repository: $REPO_DIR"
echo "Bump type: $BUMP_TYPE"
echo ""

# 1. Ensure we're on develop
echo "→ Checking branch..."
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "develop" ]; then
    echo "! Not on develop branch (currently on: $CURRENT_BRANCH)"
    read -p "Switch to develop? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        git checkout develop
        git pull origin develop
    else
        echo "Aborted. Please switch to develop branch first."
        exit 1
    fi
fi

# 2. Check current state
echo ""
echo "→ Checking current versions..."
CODE_VERSION=$(grep -E 'Version\s*=\s*"[^"]+"' cmd/*/main.go 2>/dev/null | head -1 | sed 's/.*"\([^"]*\)".*/\1/')
GIT_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "NONE")
GH_RELEASE=$(gh release list --limit 1 --json tagName -q '.[0].tagName' 2>/dev/null || echo "NONE")

echo "  Code:   $CODE_VERSION"
echo "  Git:    $GIT_TAG"
echo "  GitHub: $GH_RELEASE"

# 3. Validate alignment before proceeding
CODE_V="v$CODE_VERSION"
if [ "$CODE_V" != "$GH_RELEASE" ] && [ "$CODE_VERSION" != "$GH_RELEASE" ]; then
    echo ""
    echo "! WARNING: Starting from mismatched state!"
    echo "  Code version ($CODE_VERSION) does not match GitHub release ($GH_RELEASE)"
    echo ""
    read -p "Continue anyway? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted."
        exit 1
    fi
fi

# 4. Calculate new version
echo ""
echo "→ Calculating new version..."
LATEST="${GH_RELEASE#v}"
if [ "$LATEST" = "NONE" ]; then
    LATEST="0.0.0"
fi
NEW_VERSION=$(bash "$REPO_DIR/scripts/release/bump-version.sh" "$LATEST" "$BUMP_TYPE")
echo "  New version: v$NEW_VERSION"

# 5. Update code
echo ""
echo "→ Updating code version..."
sed -i "s/Version\s*=\s*\"[^\"]*\"/Version   = \"$NEW_VERSION\"/" cmd/*/main.go
git diff cmd/*/main.go

# 6. Update changelog
echo ""
echo "→ Updating changelog..."
TODAY=$(date +%Y-%m-%d)
if [ -f docs/changelog.md ]; then
    # Check if changelog has proper header
    if ! head -1 docs/changelog.md | grep -q "# Changelog"; then
        echo "# Changelog" > docs/changelog.md.tmp
        echo "" >> docs/changelog.md.tmp
        cat docs/changelog.md >> docs/changelog.md.tmp
        mv docs/changelog.md.tmp docs/changelog.md
    fi
    
    # Prepend new version
    TEMP=$(mktemp)
    cat > "$TEMP" << EOF
# Changelog

## [$NEW_VERSION] - $TODAY

### Changed
- Version bump to v$NEW_VERSION

EOF
    tail -n +3 docs/changelog.md >> "$TEMP"
    mv "$TEMP" docs/changelog.md
    echo "  Updated docs/changelog.md"
fi

# 7. Commit
echo ""
echo "→ Committing changes..."
git add -A
git status
echo ""
read -p "Commit these changes? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    git commit -m "chore(release): bump version to v$NEW_VERSION"
else
    echo "Aborted. Changes not committed."
    git checkout -- cmd/*/main.go 2>/dev/null || true
    exit 1
fi

# 8. Push to develop
echo ""
echo "→ Pushing to develop..."
git push origin develop

echo ""
echo "=== Release Preparation Complete ==="
echo ""
echo "Next steps:"
echo "  1. Merge develop to main:"
echo "     git checkout main && git merge develop && git push origin main"
echo "  2. Create and push tag:"
echo "     git tag -a v$NEW_VERSION -m 'Release v$NEW_VERSION'"
echo "     git push origin v$NEW_VERSION"
echo ""
echo "Or run: ./scripts/release/publish.sh v$NEW_VERSION"
