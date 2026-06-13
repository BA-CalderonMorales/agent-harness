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
USAGE
  exit 0
fi

targets=("$@")
if [[ ${#targets[@]} -eq 0 ]]; then
  targets=("${DEFAULT_TARGETS[@]}")
fi

go run github.com/zimmski/go-mutesting/cmd/go-mutesting@latest \
  --test-recursive \
  --exec-timeout "${GO_MUTESTING_TIMEOUT_SECONDS:-20}" \
  "${targets[@]}"
