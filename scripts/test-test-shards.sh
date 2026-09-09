#!/bin/bash
# test-test-shards.sh — unit tests for the test shard gate.
# Fails on the first unexpected result; prints a summary at the end.
#
# Covers the shard contract that ci.yml's test matrix depends on:
#   - every shard name resolves (list exits 0, prints packages)
#   - the three matrix shards (tui/core/rest) tile the package set:
#     every package covered, none duplicated
#   - shard isolation: no package appears in two shards
#   - the `rest` shard excludes tui and core packages
#   - unknown shard names are rejected
#   - the pkg/... regression: patterns must use the ./ prefix, and
#     `rest` must include pkg/* packages (the original drift that
#     motivated this script)

SCRIPT="$(dirname "$0")/test-shards.sh"
pass=0
fail=0

expect_ok() {
    if sh "$SCRIPT" "$@" >/dev/null 2>&1; then
        pass=$((pass + 1))
    else
        fail=$((fail + 1))
        echo "FAIL: expected OK for '$*'"
    fi
}

expect_reject() {
    if sh "$SCRIPT" "$@" >/dev/null 2>&1; then
        fail=$((fail + 1))
        echo "FAIL: expected rejection for '$*'"
    else
        pass=$((pass + 1))
    fi
}

expect_nonempty() {
    out=$(sh "$SCRIPT" "$@" 2>/dev/null)
    if [ -n "$out" ]; then
        pass=$((pass + 1))
    else
        fail=$((fail + 1))
        echo "FAIL: expected non-empty output for '$*'"
    fi
}

# The three CI matrix shards must resolve and be non-empty.
for shard in tui core rest; do
    expect_ok list "$shard"
    expect_nonempty list "$shard"
done

# Unknown names are rejected.
expect_reject list "nonexistent"
expect_reject list ""
expect_reject ""

# Coverage: complete, no overlap.
expect_ok coverage

# Shard isolation, verified independently of the coverage command:
# intersection of tui and rest must be empty.
tui_pkgs=$(sh "$SCRIPT" list tui)
overlap=$(sh "$SCRIPT" list rest | grep -Fx -f <(printf '%s\n' "$tui_pkgs") || true)
if [ -z "$overlap" ]; then
    pass=$((pass + 1))
else
    fail=$((fail + 1))
    echo "FAIL: tui/rest overlap: $overlap"
fi

# core must be disjoint from rest too.
core_pkgs=$(sh "$SCRIPT" list core)
overlap=$(sh "$SCRIPT" list rest | grep -Fx -f <(printf '%s\n' "$core_pkgs") || true)
if [ -z "$overlap" ]; then
    pass=$((pass + 1))
else
    fail=$((fail + 1))
    echo "FAIL: core/rest overlap: $overlap"
fi

# The pkg/... regression: pkg packages must land in some shard.
pkg_count=$(sh "$SCRIPT" list rest | grep -c '/pkg/' || true)
if [ "$pkg_count" -ge 5 ]; then
    pass=$((pass + 1))
else
    fail=$((fail + 1))
    echo "FAIL: expected pkg/* packages in rest shard, found $pkg_count"
fi

# The Go pattern rule this script exists to encode: a bare `pkg/...`
# (no ./ prefix) matches no packages in module mode. Expansion of a
# shard must never silently produce zero packages for a real directory.
# Note: go list exits 0 even when it matches nothing (warning only on
# stderr) — assert on stdout emptiness, not the exit code.
if [ -z "$(go list pkg/... 2>/dev/null)" ]; then
    pass=$((pass + 1))
else
    fail=$((fail + 1))
    echo "FAIL: bare pkg/... unexpectedly resolved; shard assumptions changed"
fi

echo
echo "test-test-shards: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
