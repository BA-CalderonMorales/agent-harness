#!/usr/bin/env bash
# Run a command with an RSS ceiling enforced by a bounded watchdog.
#
# The command is started in a private session. RSS and cleanup operate on the
# session leader and every currently visible descendant, so a child which
# creates another process group cannot escape the guard. Signals use explicit
# PIDs; the invoking shell is never addressed by a negative process-group ID.
#
# Usage: scripts/verify/memguard.sh <rss-limit-mb> <command> [args...]
set -u

if [ "$#" -lt 2 ]; then
	printf 'usage: %s <rss-limit-mb> <command> [args...]\n' "$0" >&2
	exit 2
fi

LIMIT_MB=$1
shift
case "$LIMIT_MB" in
	''|*[!0-9]*|0)
		printf '%s: RSS limit must be a positive integer (MB)\n' "$0" >&2
		exit 2
		;;
esac
# Keep the multiplication below within Bash's signed integer range.
if [ "${#LIMIT_MB}" -gt 16 ] || {
	[ "${#LIMIT_MB}" -eq 16 ] && [ "$LIMIT_MB" \> 9007199254740991 ]
}; then
	printf '%s: RSS limit is too large\n' "$0" >&2
	exit 2
fi

if ! command -v setsid >/dev/null 2>&1; then
	printf '%s: setsid is required to create a private process session\n' "$0" >&2
	exit 2
fi
for tool in ps awk sleep; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		printf '%s: %s is required for process-tree monitoring\n' "$0" "$tool" >&2
		exit 2
	fi
done

LIMIT_KB=$((LIMIT_MB * 1024))
PEAK_KB=0
CHILD=
KNOWN_PIDS=

# Print the root and all descendants from one ps snapshot. This is deliberately
# PID-based instead of process-group-based because descendants may call setsid.
tree_pids() {
	ps -eo pid=,ppid= 2>/dev/null | awk -v root="$1" '
		{ pid[NR] = $1; ppid[NR] = $2 }
		END {
			seen[root] = 1
			changed = 1
			while (changed) {
				changed = 0
				for (i = 1; i <= NR; i++) if (seen[ppid[i]] && !seen[pid[i]]) {
					seen[pid[i]] = 1; changed = 1
				}
			}
			for (i = 1; i <= NR; i++) if (seen[pid[i]]) print pid[i]
		}
	'
}

tree_rss_kb() {
	ps -eo pid=,ppid=,rss= 2>/dev/null | awk -v root="$1" '
		{ pid[NR] = $1; ppid[NR] = $2; rss[NR] = $3 }
		END {
			seen[root] = 1; changed = 1
			while (changed) {
				changed = 0
				for (i = 1; i <= NR; i++) if (seen[ppid[i]] && !seen[pid[i]]) {
					seen[pid[i]] = 1; changed = 1
				}
			}
			sum = 0
			for (i = 1; i <= NR; i++) if (seen[pid[i]]) sum += rss[i]
			print sum + 0
		}
	'
}

remember_tree() {
	new_pids=$(tree_pids "$1")
	[ -n "$new_pids" ] || return 0
	while IFS= read -r pid; do
		case " $KNOWN_PIDS " in
			*" $pid "*) ;;
			*) KNOWN_PIDS="${KNOWN_PIDS:+$KNOWN_PIDS }$pid" ;;
		esac
	done <<EOF
$new_pids
EOF
}

terminate_tree() {
	[ -n "${1:-}" ] || return 0
	# Snapshot before signaling: children which call setsid would otherwise be
	# reparented as soon as the root exits and disappear from PPID traversal.
	pids=$(tree_pids "$1")
	[ -n "$pids" ] && while IFS= read -r pid; do kill -TERM "$pid" 2>/dev/null || true; done <<EOF
$pids
EOF
	for pid in $KNOWN_PIDS; do kill -TERM "$pid" 2>/dev/null || true; done
	# The root's private process group catches descendants that outlive it. This
	# negative-PGID signal is safe because setsid made the group exclusively ours.
	kill -TERM -- -"$1" 2>/dev/null || true
	sleep 1
	kill -KILL -- -"$1" 2>/dev/null || true
	pids=$(tree_pids "$1")
	[ -n "$pids" ] && while IFS= read -r pid; do kill -KILL "$pid" 2>/dev/null || true; done <<EOF
$pids
EOF
	for pid in $KNOWN_PIDS; do kill -KILL "$pid" 2>/dev/null || true; done
}

cleanup() {
	[ -n "$CHILD" ] && terminate_tree "$CHILD"
}

on_signal() {
	signal=$1
	cleanup
	trap - EXIT
	exit $((128 + signal))
}

trap cleanup EXIT
trap 'on_signal 2' INT
trap 'on_signal 15' TERM

# setsid gives the root a private process group; descendant-based accounting
# still covers children that create new groups of their own.
setsid -- "$@" &
CHILD=$!

while kill -0 "$CHILD" 2>/dev/null; do
	remember_tree "$CHILD"
	TOTAL=$(tree_rss_kb "$CHILD")
	TOTAL=${TOTAL:-0}
	[ "$TOTAL" -gt "$PEAK_KB" ] && PEAK_KB=$TOTAL
	if [ "$TOTAL" -gt "$LIMIT_KB" ]; then
		printf '[memguard] RSS %dMB exceeded the %dMB ceiling; terminating launched tree\n' \
			$((TOTAL / 1024)) "$LIMIT_MB" >&2
		terminate_tree "$CHILD"
		wait "$CHILD" 2>/dev/null || true
		trap - EXIT
		printf '[memguard] peak RSS: %dMB of %dMB ceiling\n' $((PEAK_KB / 1024)) "$LIMIT_MB" >&2
		exit 137
	fi
	sleep 1
done

wait "$CHILD"
status=$?
# A normally exiting root may leave descendants; clean them without changing
# the root's exit code.
cleanup
trap - EXIT
printf '[memguard] peak RSS: %dMB of %dMB ceiling\n' $((PEAK_KB / 1024)) "$LIMIT_MB" >&2
exit "$status"
