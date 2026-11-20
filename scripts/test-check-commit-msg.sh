#!/bin/sh
# test-check-commit-msg.sh - unit tests for scripts/check-commit-msg.sh.

SCRIPT="$(cd "$(dirname "$0")" && pwd)/check-commit-msg.sh"
WORK=$(mktemp -d "${TMPDIR:-/tmp}/ah-commit-msg-test.XXXXXX")
trap 'rm -rf "$WORK"' EXIT
FAIL=0

check() {
    printf '%s\n' "$1" > "$WORK/msg"
    sh "$SCRIPT" "$WORK/msg" 2>/dev/null
}

expect_ok() {
    if check "$1"; then echo "  ok      $1"; else echo "  FAIL    rejected: $1"; FAIL=1; fi
}

expect_reject() {
    if check "$1"; then echo "  FAIL    accepted: $1"; FAIL=1; else echo "  ok      rejected: $1"; fi
}

echo "commit-msg: conventional subjects pass"
expect_ok "feat(tui): add a Labs section"
expect_ok "fix: keep the provider across a restart"
expect_ok "refactor(core)!: drop the legacy settings file"
expect_ok "ci(release/notes): scope with a slash"
expect_ok "security(web_fetch): guard redirects"
expect_ok "# a comment line git strips
chore(deps): bump x"

echo "commit-msg: git-generated subjects pass"
expect_ok "Merge pull request #53 from someone/feat/x"
expect_ok 'Revert "feat(tui): add a Labs section"'
expect_ok "fixup! feat(tui): add a Labs section"

echo "commit-msg: everything else is rejected"
expect_reject "Add Depot CI workflows"
expect_reject "feat add a thing"
expect_reject "feature(tui): not a known type"
expect_reject "Feat(tui): capitalized type"
expect_reject "feat(TUI): uppercase scope"
expect_reject "feat(): empty scope"
expect_reject "feat:  leading double space"
expect_reject "release/v0.3.8: old branch-style subject"

echo "commit-msg: range mode checks every commit and skips merges"
repo="$WORK/repo"
git init -q --initial-branch=main "$repo"
git -C "$repo" -c user.name=t -c user.email=t@example.invalid commit -q --allow-empty -m "chore: base"
base=$(git -C "$repo" rev-parse HEAD)
git -C "$repo" -c user.name=t -c user.email=t@example.invalid commit -q --allow-empty -m "feat(x): good"
if (cd "$repo" && sh "$SCRIPT" --range "$base..HEAD" >/dev/null 2>&1); then
    echo "  ok      clean range passes"
else
    echo "  FAIL    clean range rejected"; FAIL=1
fi
git -C "$repo" -c user.name=t -c user.email=t@example.invalid commit -q --allow-empty -m "wip stuff"
if (cd "$repo" && sh "$SCRIPT" --range "$base..HEAD" >/dev/null 2>&1); then
    echo "  FAIL    range with a bad commit passed"; FAIL=1
else
    echo "  ok      range with a bad commit is rejected"
fi

if [ "$FAIL" -ne 0 ]; then
    echo "commit-msg tests FAILED"
    exit 1
fi
echo "commit-msg tests passed"
