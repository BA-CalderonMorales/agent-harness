#!/bin/sh
# next-version.sh - compute the next release version.
#
# Usage:
#   git log --format=%B vX.Y.Z..origin/main | next-version.sh vX.Y.Z [auto|major|minor|patch]
#
# auto reads the conventional commit messages on stdin: a breaking change
# (type!: or a BREAKING CHANGE: footer) bumps major, feat bumps minor, and
# anything else bumps patch. Before 1.0.0 each auto step shifts down one,
# so breaking bumps minor and feat bumps patch. An explicit bump is taken
# as given and stdin is ignored.

set -eu

usage="usage: next-version.sh <last-tag> [auto|major|minor|patch]"
last="${1:?$usage}"
bump="${2:-auto}"

core=$(printf '%s\n' "$last" | sed -n 's/^v\{0,1\}\([0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\)$/\1/p')
if [ -z "$core" ]; then
    echo "next-version: '$last' is not a vX.Y.Z tag" >&2
    exit 2
fi

IFS=. read -r major minor patch <<EOF
$core
EOF

if [ "$bump" = auto ]; then
    messages=$(cat)
    if printf '%s\n' "$messages" | grep -Eq '^[a-z]+(\([^)]*\))?!:|^BREAKING[ -]CHANGE:'; then
        bump=major
    elif printf '%s\n' "$messages" | grep -Eq '^feat(\([^)]*\))?:'; then
        bump=minor
    else
        bump=patch
    fi
    if [ "$major" -eq 0 ]; then
        case "$bump" in
            major) bump=minor ;;
            minor) bump=patch ;;
        esac
    fi
fi

case "$bump" in
    major) major=$((major + 1)); minor=0; patch=0 ;;
    minor) minor=$((minor + 1)); patch=0 ;;
    patch) patch=$((patch + 1)) ;;
    *)
        echo "next-version: unknown bump '$bump'" >&2
        echo "$usage" >&2
        exit 2
        ;;
esac

echo "v$major.$minor.$patch"
