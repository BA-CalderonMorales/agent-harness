#!/bin/sh
# check-commit-msg.sh - conventional commit gate.
#
# make release reads commit messages to choose the next version, so a
# subject that is not a conventional commit would release a feature as a
# patch or miss a breaking change. This makes the rule a check, not prose.
#
# Usage: check-commit-msg.sh <message-file>       the commit-msg hook
#        check-commit-msg.sh --range <git-range>   every commit in a PR (CI)
#        check-commit-msg.sh --title <text>        a PR title, which a squash
#                                                  merge can use as the subject
#
# The hook accepts fixup!, squash!, and amend! subjects so local autosquash
# works, and "Merge ..." subjects so git's own merge commits pass. Range and
# title modes reject both: range mode already skips real merge commits, so a
# "Merge ..." subject there or in a PR title is a hand-written subject that
# hides its real type, and the release bump would miss it.

set -eu

TYPES='feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert|security'
PATTERN="^($TYPES)(\([a-z0-9][a-z0-9._/-]*\))?!?: [^ ]"

hint() {
    echo "    Use: type(scope): description   (scope optional, ! marks a breaking change)" >&2
    echo "    Types: $(printf '%s' "$TYPES" | sed 's/|/, /g')" >&2
}

# check_subject <subject> <label> <local: yes|no>
# local=yes is the commit-msg hook; local=no is range and title mode.
check_subject() {
    case "$1" in
        'Revert "'*) return 0 ;;
        "Merge "*)
            if [ "$3" = yes ]; then
                return 0
            fi
            echo "[!] 'Merge ...' is not a conventional subject${2:+ ($2)}: $1" >&2
            return 1
            ;;
        "fixup! "*|"squash! "*|"amend! "*)
            if [ "$3" = yes ]; then
                return 0
            fi
            echo "[!] unsquashed ${1%%!*}! commit${2:+ ($2)}: $1" >&2
            echo "    Squash it into the commit it fixes before this reaches main." >&2
            return 1
            ;;
    esac
    if printf '%s\n' "$1" | grep -Eq "$PATTERN"; then
        return 0
    fi
    echo "[!] not a conventional commit${2:+ ($2)}: $1" >&2
    return 1
}

case "${1:-}" in
    --range)
        range="${2:?usage: check-commit-msg.sh --range <git-range>}"
        commits=$(git rev-list --no-merges "$range") || {
            echo "[!] cannot list commits in $range" >&2
            exit 1
        }
        fail=0
        for sha in $commits; do
            subject=$(git log -1 --format=%s "$sha")
            check_subject "$subject" "$(git rev-parse --short "$sha")" no || fail=1
        done
        if [ "$fail" -ne 0 ]; then
            hint
            exit 1
        fi
        echo "[OK] every commit in $range is a conventional commit"
        ;;
    --title)
        title="${2:?usage: check-commit-msg.sh --title <text>}"
        if ! check_subject "$title" "PR title" no; then
            hint
            exit 1
        fi
        echo "[OK] PR title is a conventional commit"
        ;;
    *)
        file="${1:?usage: check-commit-msg.sh <message-file> | --range <git-range> | --title <text>}"
        subject=$(grep -v '^#' "$file" | sed '/^[[:space:]]*$/d' | head -n 1)
        # An empty message is git's to reject; it aborts the commit on its own.
        [ -n "$subject" ] || exit 0
        if ! check_subject "$subject" "" yes; then
            hint
            exit 1
        fi
        ;;
esac
