#!/bin/bash
# bump-version.sh - Calculate next version based on semver
# Usage: ./scripts/release/bump-version.sh <current_version> [patch|minor|major]

CURRENT_VERSION="${1:-}"
BUMP_TYPE="${2:-patch}"

if [ -z "$CURRENT_VERSION" ]; then
    echo "Usage: bump-version.sh <current_version> [patch|minor|major]"
    echo "Example: bump-version.sh v0.0.42 patch"
    exit 1
fi

# Strip 'v' prefix if present
VERSION="${CURRENT_VERSION#v}"

# Split into components
MAJOR=$(echo "$VERSION" | cut -d. -f1)
MINOR=$(echo "$VERSION" | cut -d. -f2)
PATCH=$(echo "$VERSION" | cut -d. -f3)

# Validate components
if ! [[ "$MAJOR" =~ ^[0-9]+$ ]] || ! [[ "$MINOR" =~ ^[0-9]+$ ]] || ! [[ "$PATCH" =~ ^[0-9]+$ ]]; then
    echo "Error: Invalid version format. Expected: MAJOR.MINOR.PATCH"
    exit 1
fi

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
