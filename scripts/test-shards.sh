#!/bin/sh
# test-shards.sh — package shards for the parallel CI test matrix.
#
# The serial "go test ./..." test job is the CI tail: the TUI package
# alone takes ~50s and every other package waits behind it. Sharding by
# package group runs the shards as GitHub Actions matrix jobs in
# parallel and turns one aggregate pass/fail into per-shard visibility.
#
# Shard contract:
#   tui    — internal/interface/tui alone (the ~50s long pole; anything
#            bundled with it just serializes behind it)
#   core   — internal/core + internal/agent (state, agent loop)
#   rest   — everything else, computed by filtering `go list ./...` so
#            new top-level dirs can never silently fall out of coverage
#            (the drift the `coverage` command guards against cannot
#            happen for rest by construction).
#
# Every command must be run from the repo root (all pattern expansion
# goes through `go list ./...`, which requires the ./ prefix — a bare
# `pkg/...` pattern matches no packages in module mode; that is the bug
# this script was refactored to fix).
#
# Usage:
#   test-shards.sh list <name>     — print one shard's packages per line
#   test-shards.sh run <name>      — run one shard's tests
#   test-shards.sh coverage        — fail if any package is missing from
#                                    every shard (drift guard)

set -u

# Resolve the repo root from the script location so calls work from any
# cwd. All commands re-root before doing anything.
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# Expand a shard's package patterns to full import paths.
# Empty patterns are skipped; failed expansions are skipped with a
# warning on stderr so one bad pattern cannot abort coverage.
expand() {
    for p in $1; do
        if out=$(cd "$ROOT" && go list "$p" 2>/dev/null); then
            echo "$out"
        else
            echo "warn: pattern '$p' matched no packages" >&2
        fi
    done
}

case "${1:-}" in
list)
    case "${2:-}" in
    tui)
        expand "./internal/interface/tui/..."
        ;;
    core)
        expand "./internal/core/... ./internal/agent/..."
        ;;
    rest)
        # Complement of tui+core over `go list ./...`. Computed, not
        # enumerated, so coverage is complete by construction.
        cd "$ROOT"
        skip=$(sh "$0" list tui; sh "$0" list core)
        go list ./... | while read -r p; do
            case "$skip" in
            *"$p"*) ;;
            *) echo "$p" ;;
            esac
        done
        ;;
    *)
        echo "unknown shard: ${2:-}" >&2
        exit 1
        ;;
    esac
    ;;
run)
    pkgs=$(sh "$0" list "${2:-}" | tr '\n' ' ')
    if [ -z "$pkgs" ]; then
        echo "unknown shard: ${2:-}" >&2
        exit 1
    fi
    cd "$ROOT"
    # shellcheck disable=SC2086
    exec go test -count=1 -timeout 15m $pkgs
    ;;
coverage)
    all=$(cd "$ROOT" && go list ./... 2>/dev/null)
    covered=$(sh "$0" list tui; sh "$0" list core; sh "$0" list rest)
    missing=0
    dup=0
    for p in $all; do
        count=$(printf '%s\n' $covered | grep -cx "$p")
        if [ "$count" -eq 0 ]; then
            echo "MISSING: $p" >&2
            missing=1
        elif [ "$count" -gt 1 ]; then
            echo "DUPLICATE: $p" >&2
            dup=1
        fi
    done
    if [ "$missing" -ne 0 ]; then
        echo "shard drift: packages above are in no shard" >&2
        exit 1
    fi
    if [ "$dup" -ne 0 ]; then
        echo "shard drift: packages above are in more than one shard" >&2
        exit 1
    fi
    echo "[OK] all $(echo "$all" | wc -l) packages covered by shards exactly once"
    ;;
*)
    echo "usage: $0 {list <shard>|run <shard>|coverage}" >&2
    exit 1
    ;;
esac
