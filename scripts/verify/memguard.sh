#!/usr/bin/env bash
# Run a command with an RSS ceiling enforced by a watchdog.
#
# GOMEMLIMIT is a soft limit: a live-heap leak (reachable data) grows
# straight past it while the GC spins, and the kernel's OOM killer takes
# the whole host — twice, via tui.test. This guard is hard: once the
# watched tree crosses MEMGUARD_RSS_MB, the process group is killed and
# the failure surfaces as a normal non-zero exit.
#
# Usage: scripts/verify/memguard.sh <mb-limit> <command> [args...]
set -u

if [ $# -lt 2 ]; then
	printf 'usage: %s <rss-limit-mb> <command> [args...]\n' "$0" >&2
	exit 2
fi

LIMIT_MB=$1
shift

LIMIT_KB=$((LIMIT_MB * 1024))
PEAK_KB=0

set +e
"$@" &
CHILD=$!
set -e
trap 'kill -TERM -- -$CHILD 2>/dev/null' EXIT

# Poll the whole process group: go test spawns the package binary as a
# grandchild, and that grandchild is the one that balloons.
while kill -0 "$CHILD" 2>/dev/null; do
	TOTAL=$(ps -eo pgid=,rss= | awk -v g="$(ps -o pgid= -p "$CHILD" 2>/dev/null | tr -d ' ')" \
		'$1 == g { sum += $2 } END { print sum + 0 }')
	if [ "${TOTAL:-0}" -gt "$PEAK_KB" ]; then
		PEAK_KB=$TOTAL
	fi
	if [ "${TOTAL:-0}" -gt "$LIMIT_KB" ]; then
		printf '[memguard] RSS %dMB exceeded the %dMB ceiling; killing process group\n' \
			$((TOTAL / 1024)) "$LIMIT_MB" >&2
		kill -TERM -- -"$CHILD" 2>/dev/null
		sleep 2
		kill -KILL -- -"$CHILD" 2>/dev/null
		exit 137
	fi
	sleep 1
done

wait "$CHILD"
status=$?
trap - EXIT
printf '[memguard] peak RSS: %dMB of %dMB ceiling\n' $((PEAK_KB / 1024)) "$LIMIT_MB" >&2
exit $status
