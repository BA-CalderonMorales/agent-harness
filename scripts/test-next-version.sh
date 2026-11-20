#!/bin/sh
# test-next-version.sh - unit tests for scripts/release/next-version.sh.

SCRIPT="$(dirname "$0")/release/next-version.sh"
FAIL=0

# records <message>... - one stdin record per commit, the shape that
# git log --format='%x1e%s%n%b' produces: \036 before each subject.
records() {
    for message in "$@"; do
        printf '\036%s\n' "$message"
    done
}

# expect <want> <last-tag> <bump> <commit message>...
expect() {
    want="$1" last="$2" bump="$3"
    shift 3
    got=$(records "$@" | sh "$SCRIPT" "$last" "$bump" 2>/dev/null)
    if [ "$got" = "$want" ]; then
        echo "  ok    $last $bump -> $got"
    else
        echo "  FAIL  $last $bump -> got '$got', want '$want' (commits: $*)"
        FAIL=1
    fi
}

expect_error() {
    last="$1" bump="$2"
    if records "fix: x" | sh "$SCRIPT" "$last" "$bump" >/dev/null 2>&1; then
        echo "  FAIL  $last $bump should be rejected"
        FAIL=1
    else
        echo "  ok    $last $bump rejected"
    fi
}

echo "next-version: before 1.0.0, auto steps shift down one"
expect v0.4.2 v0.4.1 auto "fix(tui): keep the cursor row"
expect v0.4.2 v0.4.1 auto "feat(tui): add a Labs section"
expect v0.5.0 v0.4.1 auto "feat(config)!: drop the legacy settings file"
expect v0.5.0 v0.4.1 auto "fix: rename a flag

BREAKING CHANGE: --old is gone"
expect v0.4.2 v0.4.1 auto "Merge pull request #53 from someone/branch"

echo "next-version: from 1.0.0, auto follows semver"
expect v1.2.4 v1.2.3 auto "fix: one"
expect v1.3.0 v1.2.3 auto "fix: one" "feat(agent): two"
expect v2.0.0 v1.2.3 auto "refactor(core)!: three"

echo "next-version: only subjects declare feat or a ! marker"
expect v1.2.4 v1.2.3 auto "docs: explain commit syntax

For example:
feat!: remove an option"
expect v1.2.4 v1.2.3 auto "docs: show a feature commit

feat(tui): an example subject in a body"
expect v2.0.0 v1.2.3 auto "docs: note the removal

BREAKING-CHANGE: the footer still counts"

echo "next-version: an explicit bump is taken as given"
expect v1.0.0 v0.4.1 major "fix: anything"
expect v0.5.0 v0.4.1 minor "fix: anything"
expect v0.4.2 v0.4.1 patch "feat!: anything"
expect v0.4.2 0.4.1 auto "fix: tags without the v prefix parse too"

echo "next-version: bad input is rejected"
expect_error v0.4 auto
expect_error v0.4.1-rc.1 auto
expect_error release/0.4.1 auto
expect_error v0.4.1 sideways

if [ "$FAIL" -ne 0 ]; then
    echo "next-version tests FAILED"
    exit 1
fi
echo "next-version tests passed"
