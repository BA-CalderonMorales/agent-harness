#!/usr/bin/env bash
#
# verify-structure: enforce the one-concept-per-file house rule.
#
# Soft guidance by default: reports every non-test Go file over the line
# budget. Pass --strict to fail the build on any over-budget file (use in
# CI when you need the gate hard).

set -euo pipefail

BUDGET=400
STRICT=0
if [[ "${1:-}" == "--strict" ]]; then
	STRICT=1
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
over=0
total=0

while IFS= read -r -d '' file; do
	total=$((total + 1))
	lines=$(wc -l <"$file")
	if ((lines > BUDGET)); then
		over=$((over + 1))
		printf '  [%s] %4d lines  %s\n' "over" "$lines" "${file#"$ROOT"/}"
	fi
done < <(find "$ROOT" -name '*.go' -not -name '*_test.go' -not -path '*/build/*' -not -path '*/.git/*' -print0)

if ((over == 0)); then
	printf '[ok] structure: %d source files, all within %d lines\n' "$total" "$BUDGET"
	exit 0
fi

printf '[warn] structure: %d files over the %d-line budget (test/spec files exempt, soft guidance)\n' "$over" "$BUDGET"
if ((STRICT == 1)); then
	printf '[fail] structure: over-budget files found (strict mode)\n'
	exit 1
fi
exit 0
