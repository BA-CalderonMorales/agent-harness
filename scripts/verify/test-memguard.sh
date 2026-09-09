#!/usr/bin/env bash
# Focused regression checks for memguard's process-tree contract.
set -u

SCRIPT=$(dirname "$0")/memguard.sh
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
pass=0
fail=0

expect_status() {
	name=$1
	want=$2
	shift 2
	if "$@" >/dev/null 2>"$tmp/$name.err"; then
		got=0
	else
		got=$?
	fi
	if [ "$got" -eq "$want" ]; then
		pass=$((pass + 1))
	else
		fail=$((fail + 1))
		echo "FAIL: $name expected status $want, got $got"
		sed -n '1,20p' "$tmp/$name.err"
	fi
}

expect_status missing-limit 2 bash "$SCRIPT"
expect_status bad-limit 2 bash "$SCRIPT" nope true
expect_status zero-limit 2 bash "$SCRIPT" 0 true
expect_status negative-limit 2 bash "$SCRIPT" -1 true
expect_status missing-command 2 bash "$SCRIPT" 64
expect_status exit-propagation 7 bash "$SCRIPT" 64 sh -c 'exit 7'
expect_status oversized-limit 2 bash "$SCRIPT" 999999999999999999 true

# A missing monitor utility must fail closed instead of running unguarded.
mkdir "$tmp/no-ps-bin"
ln -s "$(command -v setsid)" "$tmp/no-ps-bin/setsid"
expect_status missing-ps 2 env PATH="$tmp/no-ps-bin" /bin/bash "$SCRIPT" 64 true

# A different process group must still be part of the guarded tree. A 1MB
# ceiling deterministically exceeds the tiny shell process without allocating.
grandchild_file=$tmp/grandchild.pid
bash "$SCRIPT" 1 sh -c 'setsid sleep 30 & echo $! > "$1"; wait' sh "$grandchild_file" \
	>"$tmp/limit.out" 2>"$tmp/limit.err" &
guard=$!
for _ in 1 2 3 4 5; do
	[ -s "$grandchild_file" ] && break
	sleep 0.1
done
wait "$guard"
limit_status=$?
grandchild=$(sed -n '1p' "$grandchild_file")
if [ "$limit_status" -eq 137 ] && grep -q 'exceeded' "$tmp/limit.err" && \
	[ -n "$grandchild" ] && ! kill -0 "$grandchild" 2>/dev/null; then
	pass=$((pass + 1))
else
	fail=$((fail + 1))
	echo "FAIL: limit cleanup status=$limit_status grandchild=${grandchild:-missing}"
	[ -n "$grandchild" ] && kill "$grandchild" 2>/dev/null || true
fi

# A normally exiting root's status must survive cleanup of its lingering child.
normal_file=$tmp/normal.pid
bash "$SCRIPT" 64 sh -c 'sleep 30 & echo $! > "$1"; exit 7' sh "$normal_file" \
	>"$tmp/normal.out" 2>"$tmp/normal.err"
normal_status=$?
normal_child=$(sed -n '1p' "$normal_file")
if [ "$normal_status" -eq 7 ] && [ -n "$normal_child" ] && \
	! kill -0 "$normal_child" 2>/dev/null; then
	pass=$((pass + 1))
else
	fail=$((fail + 1))
	echo "FAIL: normal cleanup status=$normal_status child=${normal_child:-missing}"
	[ -n "$normal_child" ] && kill "$normal_child" 2>/dev/null || true
fi

# A detached descendant can be reparented when the root exits. The guard must
# retain the PID it observed and clean it up without touching this shell.
detached_file=$tmp/detached.pid
bash "$SCRIPT" 64 sh -c 'setsid sleep 30 & echo $! > "$1"; sleep 2; exit 7' sh "$detached_file" \
	>"$tmp/detached.out" 2>"$tmp/detached.err"
detached_status=$?
detached_child=$(sed -n '1p' "$detached_file")
if [ "$detached_status" -eq 7 ] && [ -n "$detached_child" ] && \
	! kill -0 "$detached_child" 2>/dev/null; then
	pass=$((pass + 1))
else
	fail=$((fail + 1))
	echo "FAIL: detached cleanup status=$detached_status child=${detached_child:-missing}"
	[ -n "$detached_child" ] && kill "$detached_child" 2>/dev/null || true
fi

# A sibling of the guard is outside its private session and must survive.
sleep 30 &
sibling=$!
bash "$SCRIPT" 64 true >"$tmp/sibling.out" 2>"$tmp/sibling.err"
sibling_status=$?
if [ "$sibling_status" -eq 0 ] && kill -0 "$sibling" 2>/dev/null; then
	pass=$((pass + 1))
else
	fail=$((fail + 1))
	echo "FAIL: sibling survival status=$sibling_status sibling=$sibling"
fi
kill "$sibling" 2>/dev/null || true
wait "$sibling" 2>/dev/null || true

# Interrupting the guard cleans its tree and leaves this invoking shell alive.
signal_file=$tmp/signal.pid
bash "$SCRIPT" 64 sh -c 'sleep 30 & echo $! > "$1"; wait' sh "$signal_file" &
guard=$!
for _ in 1 2 3 4 5; do
	[ -s "$signal_file" ] && break
	sleep 0.1
done
signal_child=$(sed -n '1p' "$signal_file")
kill -TERM "$guard"
wait "$guard"
signal_status=$?
printf '%s\n' survived >"$tmp/survived"
if [ "$signal_status" -eq 143 ] && [ -s "$tmp/survived" ] && \
	[ -n "$signal_child" ] && ! kill -0 "$signal_child" 2>/dev/null; then
	pass=$((pass + 1))
else
	fail=$((fail + 1))
	echo "FAIL: signal cleanup status=$signal_status child=${signal_child:-missing}"
	[ -n "$signal_child" ] && kill "$signal_child" 2>/dev/null || true
fi

echo "memguard tests: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
