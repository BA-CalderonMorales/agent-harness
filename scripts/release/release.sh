#!/bin/sh
# release.sh - tag origin/main as the next version and push the tag.
#
# Usage: scripts/release/release.sh [--dry-run] [--yes] [--bump major|minor|patch]
#                                   [--version vX.Y.Z] [--skip-ci-check]
#
# A release is a tag on origin/main, nothing more. This script never tags
# the local checkout, so it can run from any branch at any moment. It finds
# the last release tag reachable from origin/main, computes the next version
# from the conventional commits since then (scripts/release/next-version.sh),
# requires the CI run for that commit to have passed, and pushes the tag.
# release.yml then builds and publishes the binaries.

set -eu

usage() {
    sed -n '4,5p' "$0" | sed 's/^# //'
}

die() {
    echo "release: $*" >&2
    exit 1
}

dry_run=false
assume_yes=false
bump=auto
version=""
check_ci=true

while [ $# -gt 0 ]; do
    case "$1" in
        --dry-run) dry_run=true ;;
        --yes) assume_yes=true ;;
        --bump) shift; bump="${1:-}" ;;
        --version) shift; version="${1:-}" ;;
        --skip-ci-check) check_ci=false ;;
        -h|--help) usage; exit 0 ;;
        *) usage >&2; die "unknown argument '$1'" ;;
    esac
    shift
done

script_dir=$(CDPATH= cd "$(dirname "$0")" && pwd)

git fetch --quiet --tags origin main || die "cannot fetch origin main"
target=$(git rev-parse --verify --quiet 'origin/main^{commit}') || die "origin/main does not exist"
short=$(git rev-parse --short "$target")

last=$(git describe --tags --abbrev=0 --match 'v[0-9]*' "$target" 2>/dev/null || true)
if [ -n "$last" ]; then
    range="$last..$target"
    base="$last"
else
    range="$target"
    base="v0.0.0"
fi

count=$(git rev-list --count "$range")
if [ "$count" -eq 0 ]; then
    echo "Nothing to release: origin/main ($short) is already $last."
    exit 0
fi

if [ -n "$version" ]; then
    printf '%s\n' "$version" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-(alpha|beta|rc)[0-9]*(\.[0-9A-Za-z]+)*)?$' \
        || die "--version must look like v1.2.3 or v1.2.3-rc.1, got '$version'"
    next="$version"
else
    next=$(git log --format='%x1e%s%n%b' "$range" | sh "$script_dir/next-version.sh" "$base" "$bump") \
        || die "could not compute the next version"
fi

if git rev-parse --verify --quiet "refs/tags/$next" >/dev/null; then
    die "tag $next already exists"
fi

if [ "$check_ci" = true ]; then
    command -v gh >/dev/null 2>&1 || die "gh is required to check CI (or pass --skip-ci-check)"
    ci=$(gh run list --commit "$target" --workflow ci.yml --limit 1 \
        --json status,conclusion --jq '.[0] | "\(.status) \(.conclusion)"' 2>/dev/null || true)
    case "$ci" in
        "completed success") ;;
        ""|"null null") die "no CI run found for origin/main ($short); wait for CI or pass --skip-ci-check" ;;
        completed*) die "CI for origin/main ($short) did not pass: ${ci#completed }" ;;
        *) die "CI for origin/main ($short) is still running; try again when it finishes" ;;
    esac
fi

echo "Release $next from origin/main ($short), ${last:-no earlier tag} + $count commit(s):"
git log --format='  %h %s' "$range" | head -n 40
[ "$count" -gt 40 ] && echo "  ... and $((count - 40)) more"

if [ "$dry_run" = true ]; then
    echo "Dry run: nothing tagged or pushed."
    exit 0
fi

if [ "$assume_yes" != true ]; then
    [ -t 0 ] || die "not a terminal; pass --yes to release without a prompt"
    printf 'Tag %s as %s and push it? [y/N] ' "$short" "$next"
    read -r answer
    case "$answer" in
        y|Y|yes|YES) ;;
        *) echo "Cancelled."; exit 1 ;;
    esac
fi

git tag -a "$next" -m "$next" "$target"
git push --quiet origin "refs/tags/$next" || {
    git tag -d "$next" >/dev/null
    die "pushing $next failed; the local tag was removed"
}
echo "Pushed $next. release.yml now builds and publishes it."
