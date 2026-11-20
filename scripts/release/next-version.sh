#!/bin/sh
# next-version.sh - compute the next release version.
#
# Usage:
#   git log --format='%x1e%s%n%b' vX.Y.Z..origin/main | next-version.sh vX.Y.Z [auto|major|minor|patch]
#
# stdin holds one record per commit: a line that starts with the ASCII
# record separator (\036) is a commit subject, and the lines after it are
# that commit's body. auto reads conventional commits from those records:
# a subject with a ! breaking marker, or a BREAKING CHANGE: footer in a
# body, bumps major; a feat subject bumps minor; anything else bumps patch.
# Header-like text inside a body never counts. Before 1.0.0 each auto step
# shifts down one, so breaking bumps minor and feat bumps patch. An explicit
# bump is taken as given and stdin is ignored.

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
    bump=$(awk -v rs="$(printf '\036')" '
        index($0, rs) == 1 {
            subject = substr($0, length(rs) + 1)
            if (subject ~ /^[a-z]+(\([^)]*\))?!:/) breaking = 1
            else if (subject ~ /^feat(\([^)]*\))?:/) feature = 1
            next
        }
        /^BREAKING[ -]CHANGE:/ { breaking = 1 }
        END { print (breaking ? "major" : feature ? "minor" : "patch") }
    ')
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
