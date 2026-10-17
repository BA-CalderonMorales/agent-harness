#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$PROJECT_ROOT"

DEFAULT_TARGETS=(
  "./cmd/agent-harness"
  "./internal/core/state"
  "./internal/runtime/tools"
  "./pkg/types"
)

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  cat <<'USAGE'
Usage: scripts/verify/mutation-test.sh [target...]

Runs go-mutesting against behavior-critical packages. Without arguments, it
targets command export/delegate code, session state, tool orchestration, and
public types.

Examples:
  scripts/verify/mutation-test.sh
  scripts/verify/mutation-test.sh ./internal/core/state
  scripts/verify/mutation-test.sh ./pkg/types ./internal/runtime/tools

GO_MUTESTING_MATCH restricts mutation to matching function names.
Run in a disposable checkout: the runner temporarily edits source files.
USAGE
  exit 0
fi

targets=("$@")
if [[ ${#targets[@]} -eq 0 ]]; then
  targets=("${DEFAULT_TARGETS[@]}")
fi

tool_dir="$PROJECT_ROOT/.workspace/mutation-tools"
mkdir -p "$tool_dir"
tool_binary="$tool_dir/go-mutesting"
if [[ "${OS:-}" == "Windows_NT" ]]; then
  tool_binary="$tool_binary.exe"
fi
# Keep the legacy runner's updated Go package loader pinned separately from
# application dependencies. Its original loader panics with current Go.
(cd "$SCRIPT_DIR/mutation-tools/osutil" && go test ./...)
(cd "$SCRIPT_DIR/mutation-tools" && go build -mod=readonly -o "$tool_binary" github.com/zimmski/go-mutesting/cmd/go-mutesting)
options=()
if [[ -n "${GO_MUTESTING_MATCH:-}" ]]; then
  options+=(--match "$GO_MUTESTING_MATCH")
fi
result_log="$(mktemp)"
trap 'rm -f "$result_log"' EXIT
# The upstream runner exits zero even when mutants survive. Preserve its log
# and make survivors (or an empty selection) fail this verification command.
if ! "$tool_binary" "${options[@]}" \
  --blacklist "$SCRIPT_DIR/mutation-equivalents.txt" \
  --test-recursive \
  --exec-timeout "${GO_MUTESTING_TIMEOUT_SECONDS:-20}" \
  "${targets[@]}" | tee "$result_log"; then
  exit 1
fi
if grep -q '^FAIL ' "$result_log"; then
  printf '\n[fail] Surviving mutants detected.\n' >&2
  exit 1
fi
if ! grep -Eq 'total is [1-9][0-9]*\)' "$result_log"; then
  printf '\n[fail] No mutations were verified. Check targets and GO_MUTESTING_MATCH.\n' >&2
  exit 1
fi
