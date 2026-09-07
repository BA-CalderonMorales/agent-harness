#!/bin/sh
# test-check-branch-name.sh — unit tests for the branch naming gate.
# Fails on the first unexpected result; prints a summary at the end.

SCRIPT="$(dirname "$0")/check-branch-name.sh"
pass=0
fail=0

expect_ok() {
    if sh "$SCRIPT" "$1" >/dev/null 2>&1; then
        pass=$((pass + 1))
    else
        fail=$((fail + 1))
        echo "FAIL: expected OK for '$1'"
    fi
}

expect_reject() {
    if sh "$SCRIPT" "$1" >/dev/null 2>&1; then
        fail=$((fail + 1))
        echo "FAIL: expected rejection for '$1'"
    else
        pass=$((pass + 1))
    fi
}

# Valid release branches
expect_ok "release/0.3.26"
expect_ok "release/1.0.0"
expect_ok "release/0.0.1"

# Invalid release branches
expect_reject "release/0.3.26-tui-polish"
expect_reject "release/0.3.26-polish-2"
expect_reject "release/0.3.26-"
expect_reject "release/"
expect_reject "release/0.3"
expect_reject "release/0.3.26.1"
expect_reject "release/v0.3.26"
expect_reject "release/01.3.26-extra"

# Non-release branches pass untouched
expect_ok "feature/x"
expect_ok "develop"
expect_ok "main"
expect_ok "hotfix/critical-fix"

echo "branch-name tests: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
