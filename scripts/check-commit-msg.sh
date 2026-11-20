#!/bin/sh
# check-commit-msg.sh - conventional commit gate.
#
# make release reads commit messages to choose the next version, so a
# subject that is not a conventional commit would release a feature as a
# patch or miss a breaking change. This makes the rule a check, not prose.
#
# Usage: check-commit-msg.sh <message-file>       the commit-msg hook
#        check-commit-msg.sh --range <git-range>   every commit in a PR (CI)

set -eu

TYPES='feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert|security'
PATTERN="^($TYPES)(\([a-z0-9][a-z0-9._/-]*\))?!?: [^ ]"

hint() {
    echo "    Use: type(scope): description   (scope optional, ! marks a breaking change)" >&2
    echo "    Types: $(printf '%s' "$TYPES" | sed 's/|/, /g')" >&2
}

# check_subject <subject> [label]
check_subject() {
    case "$1" in
        "Merge "*|'Revert "'*|"fixup! "*|"squash! "*|"amend! "*) return 0 ;;
    esac
    if printf '%s\n' "$1" | grep -Eq "$PATTERN"; then
        return 0
    fi
    echo "[!] not a conventional commit${2:+ ($2)}: $1" >&2
    return 1
}

if [ "${1:-}" = "--range" ]; then
    range="${2:?usage: check-commit-msg.sh --range <git-range>}"
    fail=0
    for sha in $(git rev-list --no-merges "$range"); do
        subject=$(git log -1 --format=%s "$sha")
        check_subject "$subject" "$(git rev-parse --short "$sha")" || fail=1
    done
    if [ "$fail" -ne 0 ]; then
        hint
        exit 1
    fi
    echo "[OK] every commit in $range is a conventional commit"
    exit 0
fi

file="${1:?usage: check-commit-msg.sh <message-file> | --range <git-range>}"
subject=$(grep -v '^#' "$file" | sed '/^[[:space:]]*$/d' | head -n 1)
# An empty message is git's to reject; it aborts the commit on its own.
[ -n "$subject" ] || exit 0
if ! check_subject "$subject"; then
    hint
    exit 1
fi
