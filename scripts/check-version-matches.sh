#!/bin/sh
# check-version-matches.sh — version-integrity gate.
#
# v0.3.25/v0.3.26 shipped binaries reporting 0.3.24 because tags were cut
# without bump commits. This script makes the comparison explicit so CI
# (release-branch PRs and tag pushes) can fail fast instead of shipping a
# mislabeled binary.
#
# Usage: check-version-matches.sh <expected-version> [main-go-path]
#   <expected-version>  version WITHOUT the v prefix (e.g. 0.3.28)
#   exit 0 — main.go Version equals <expected-version>
#   exit 1 — mismatch or unparsable

EXPECTED="$1"
MAIN_GO="${2:-cmd/agent-harness/main.go}"

if [ -z "$EXPECTED" ]; then
    echo "usage: $0 <expected-version> [main-go-path]" >&2
    exit 1
fi

if [ ! -f "$MAIN_GO" ]; then
    echo "[!] ERROR: $MAIN_GO not found" >&2
    exit 1
fi

# Version line shape: Version   = "0.3.24" (const block in main.go).
CODE_VERSION=$(grep -E '^[[:space:]]*Version[[:space:]]*=[[:space:]]*"[^"]+"' "$MAIN_GO" | head -1 | sed 's/.*"\([^"]*\)".*/\1/')

if [ -z "$CODE_VERSION" ]; then
    echo "[!] ERROR: could not parse Version from $MAIN_GO" >&2
    exit 1
fi

if [ "$CODE_VERSION" = "$EXPECTED" ]; then
    echo "[OK] version aligned: $CODE_VERSION"
    exit 0
fi

echo "[!] ERROR: version mismatch" >&2
echo "    expected (branch/tag): $EXPECTED" >&2
echo "    main.go Version:       $CODE_VERSION" >&2
echo "    Bump the version before releasing (scripts/release/bump-version.sh)." >&2
exit 1
