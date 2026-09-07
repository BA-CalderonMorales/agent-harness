#!/bin/sh
# test-check-version-matches.sh — unit tests for the version-integrity
# gate, run against fixtures (never the real main.go, which changes with
# every bump).

SCRIPT="$(dirname "$0")/check-version-matches.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

pass=0
fail=0

write_fixture() {
    printf 'package main\n\nconst (\n\tVersion   = "%s"\n)\n' "$1" > "$TMP/main.go"
}

expect_ok() {
    write_fixture "$1"
    if sh "$SCRIPT" "$2" "$TMP/main.go" >/dev/null 2>&1; then
        pass=$((pass + 1))
    else
        fail=$((fail + 1))
        echo "FAIL: expected OK (main=$1 expected=$2)"
    fi
}

expect_reject() {
    write_fixture "$1"
    if sh "$SCRIPT" "$2" "$TMP/main.go" >/dev/null 2>&1; then
        fail=$((fail + 1))
        echo "FAIL: expected rejection (main=$1 expected=$2)"
    else
        pass=$((pass + 1))
    fi
}

# Aligned
expect_ok "0.3.28" "0.3.28"
expect_ok "1.0.0"  "1.0.0"

# Mismatch — the v0.3.25/0.3.26 failure mode (tag newer than code)
expect_reject "0.3.24" "0.3.25"
expect_reject "0.3.24" "0.3.26"
expect_reject "0.3.27" "0.3.28"

# Code ahead of tag is also a mismatch
expect_reject "0.3.29" "0.3.28"

# v-prefix handling: expected comes without v; a v-prefixed expectation
# must not match (guards against callers passing the raw tag name)
expect_reject "0.3.28" "v0.3.28"

# Unparsable fixture
printf 'package main\n\nconst Name = "nope"\n' > "$TMP/main.go"
if sh "$SCRIPT" "0.3.28" "$TMP/main.go" >/dev/null 2>&1; then
    fail=$((fail + 1)); echo "FAIL: expected rejection (unparsable main.go)"
else
    pass=$((pass + 1))
fi

# Missing file
if sh "$SCRIPT" "0.3.28" "$TMP/absent.go" >/dev/null 2>&1; then
    fail=$((fail + 1)); echo "FAIL: expected rejection (missing file)"
else
    pass=$((pass + 1))
fi

echo "version-gate tests: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
