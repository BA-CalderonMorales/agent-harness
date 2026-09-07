#!/bin/sh
# check-branch-name.sh — release-branch naming gate.
#
# Releases ship from branches named exactly release/X.Y.Z (semver). Free-
# form suffixes (release/0.3.26-tui-polish, release/0.3.26-polish-2) broke
# the version-integrity story in 0.3.25/0.3.26: the tag derived from the
# branch no longer matched what shipped.
#
# Usage: check-branch-name.sh <branch-name>
#   exit 0 — branch passes (or is not a release branch; non-release
#            branches are untouched)
#   exit 1 — release branch with an invalid name

BRANCH="$1"

if [ -z "$BRANCH" ]; then
    echo "usage: $0 <branch-name>" >&2
    exit 1
fi

# Only release/* branches are gated; everything else passes untouched.
case "$BRANCH" in
    release/*) ;;
    *) exit 0 ;;
esac

if echo "$BRANCH" | grep -Eq '^release/[0-9]+\.[0-9]+\.[0-9]+$'; then
    echo "[OK] release branch name: $BRANCH"
    exit 0
fi

echo "[!] ERROR: release branches must be named exactly release/X.Y.Z (semver)." >&2
echo "    Got: $BRANCH" >&2
echo "    Examples of rejected names:" >&2
echo "      release/0.3.26-tui-polish  (no free-form suffixes)" >&2
echo "      release/0.3.26-polish-2    (no ordinals)" >&2
echo "      release/                   (no empty version)" >&2
exit 1
