#!/bin/bash
set -euo pipefail

# Local version bump script for agent-harness
# Usage: ./scripts/bump-version.sh [patch|minor|major|x.y.z]

MAIN_GO="cmd/agent-harness/main.go"
CHANGELOG="docs/changelog.md"

if [ $# -lt 1 ]; then
    echo "Usage: $0 [patch|minor|major|x.y.z]"
    exit 1
fi

INPUT="$1"
CURRENT=$(grep 'Version\s*=' "$MAIN_GO" | sed 's/.*"\(.*\)".*/\1/')

if [[ "$INPUT" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    NEW_VERSION="$INPUT"
else
    MAJOR=$(echo "$CURRENT" | cut -d. -f1)
    MINOR=$(echo "$CURRENT" | cut -d. -f2)
    PATCH=$(echo "$CURRENT" | cut -d. -f3)

    case "$INPUT" in
        major) MAJOR=$((MAJOR + 1)); MINOR=0; PATCH=0 ;;
        minor) MINOR=$((MINOR + 1)); PATCH=0 ;;
        patch|*) PATCH=$((PATCH + 1)) ;;
    esac

    NEW_VERSION="${MAJOR}.${MINOR}.${PATCH}"
fi

TAG="v${NEW_VERSION}"
DATE=$(date +%Y-%m-%d)

echo "Bumping version: $CURRENT -> $NEW_VERSION"

# Update main.go
sed -i "s/Version   = \".*\"/Version   = \"${NEW_VERSION}\"/" "$MAIN_GO"

# Update changelog
TMP=$(mktemp)
{
    echo "# Changelog"
    echo ""
    echo "## [${NEW_VERSION}] - ${DATE}"
    echo ""
    echo "### Changed"
    echo "- Version bump to ${TAG}"
    echo ""
    tail -n +3 "$CHANGELOG"
} > "$TMP"
mv "$TMP" "$CHANGELOG"

# Commit and tag
git add "$MAIN_GO" "$CHANGELOG"
git commit -m "chore(release): bump version to ${TAG}"
git tag "$TAG"

echo ""
echo "Created commit and tag ${TAG}"
echo "Run: git push && git push origin ${TAG}"
