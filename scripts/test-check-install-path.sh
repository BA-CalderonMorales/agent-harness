#!/bin/sh
# test-check-install-path.sh - unit tests for scripts/check-install-path.sh.

SCRIPT="$(cd "$(dirname "$0")" && pwd)/check-install-path.sh"
WORK=$(mktemp -d "${TMPDIR:-/tmp}/ah-install-path-test.XXXXXX")
trap 'rm -rf "$WORK"' EXIT
FAIL=0
NAME=agent-harness

# fake_bin DIR creates DIR with an executable $NAME inside it.
fake_bin() {
    mkdir -p "$1"
    printf '#!/bin/sh\necho fake\n' > "$1/$NAME"
    chmod +x "$1/$NAME"
}

# run_check PATH BINDIR runs the check with PATH set and captures
# stdout+stderr in $OUT and the exit status in $STATUS.
run_check() {
    OUT=$(PATH="$1" /bin/sh "$SCRIPT" "$2" "$NAME" 2>&1)
    STATUS=$?
}

pass() { echo "  ok      $1"; }
fail() { echo "  FAIL    $1"; echo "$OUT" | sed 's/^/          /'; FAIL=1; }

expect_silent() {
    run_check "$2" "$3"
    if [ "$STATUS" -eq 0 ] && [ -z "$OUT" ]; then pass "$1"; else fail "$1 (status $STATUS)"; fi
}

# expect_warn LABEL PATH BINDIR NEEDLE...: exit 0, a [warn], every needle present.
expect_warn() {
    label=$1 path=$2 bindir=$3
    shift 3
    run_check "$path" "$bindir"
    ok=1
    [ "$STATUS" -eq 0 ] || ok=0
    case $OUT in *"[warn]"*) ;; *) ok=0 ;; esac
    for needle in "$@"; do
        case $OUT in *"$needle"*) ;; *) ok=0 ;; esac
    done
    if [ "$ok" -eq 1 ]; then pass "$label"; else fail "$label (status $STATUS)"; fi
}

# expect_no_gobin LABEL: the last warning suggests no fixed install
# directory, since any such directory can itself be shadowed. The only
# placement it may suggest is the install dir first on PATH.
expect_no_gobin() {
    case $OUT in
        *GOBIN=*|*"make install"*) fail "$1" ;;
        *) pass "$1" ;;
    esac
}

install="$WORK/go/bin"
old="$WORK/local/bin"
other="$WORK/other"
fake_bin "$install"
fake_bin "$old"
mkdir -p "$other"
SYS=/usr/bin:/bin

echo "install-path: installed binary is first on PATH"
expect_silent "first on PATH" "$install:$SYS" "$install"
expect_silent "first on PATH after a dir without it" "$other:$install:$old" "$install"
expect_silent "trailing slash on the bindir" "$install:$SYS" "$install/"
expect_silent "trailing slashes on the PATH entry" "$install//:$SYS" "$install"

echo "install-path: bindir missing from PATH"
expect_warn "not on PATH" "$other:$SYS" "$install" \
    "$install" "export PATH='$install':\"\$PATH\""
expect_warn "not on PATH, an old binary runs instead" "$old:$SYS" "$install" \
    "$install" "$old/$NAME" "export PATH='$install':\"\$PATH\""
expect_no_gobin "not on PATH advice names no fixed install dir"

echo "install-path: an earlier binary shadows the installed one"
expect_warn "shadowed" "$old:$install:$SYS" "$install" \
    "$old/$NAME" "$install/$NAME" "remove or replace $old/$NAME" \
    "put $install ahead of $old on PATH" "export PATH='$install':\"\$PATH\""
expect_no_gobin "shadowed advice names no fixed install dir"

echo "install-path: a version-manager shim shadows the installed one"
shims="$WORK/mise/shims"
mkdir -p "$shims" "$WORK/usr/bin"
printf '#!/bin/sh\necho mise\n' > "$WORK/usr/bin/mise"
chmod +x "$WORK/usr/bin/mise"
ln -s "$WORK/usr/bin/mise" "$shims/$NAME"
expect_warn "shim dir earlier on PATH" "$shims:$old:$install:$SYS" "$install" \
    "$shims/$NAME is a shim from a version manager" \
    "put $install ahead of $shims on PATH" "export PATH='$install':\"\$PATH\""
expect_no_gobin "shim advice does not suggest installing into the shims dir"
case $OUT in *"remove or replace"*) fail "shim advice suggests replacing the shim" ;; *) pass "shim advice does not suggest replacing the shim" ;; esac
mkdir -p "$WORK/asdf/bin"
ln -s "$WORK/usr/bin/mise" "$WORK/asdf/bin/$NAME"
expect_warn "symlink to a differently named program is a shim" \
    "$WORK/asdf/bin:$install:$SYS" "$install" "is a shim from a version manager"

echo "install-path: empty PATH entries do not crash it"
expect_silent "leading, trailing, and double colons" ":$install::$SYS:" "$install"
expect_warn "empty entries, not on PATH" "::$other:" "$install" "$install"
expect_warn "PATH is empty" "" "$install" "$install"

echo "install-path: a symlink to the installed binary is not a shadow"
mkdir -p "$WORK/link"
ln -s "$install/$NAME" "$WORK/link/$NAME"
expect_silent "symlinked earlier on PATH" "$WORK/link:$install:$SYS" "$install"

echo "install-path: relative PATH entries and CDPATH"
# A relative PATH entry resolves against the current directory, never
# through CDPATH, even when CDPATH holds a directory of the same name.
mkdir -p "$WORK/rel/project/bin" "$WORK/cdpath/bin"
fake_bin "$WORK/rel/project/bin"
fake_bin "$WORK/cdpath/bin"
OUT=$(cd "$WORK/rel/project" && CDPATH="$WORK/cdpath" PATH="bin:$SYS" /bin/sh "$SCRIPT" "$WORK/rel/project/bin" "$NAME" 2>&1)
if [ -z "$OUT" ]; then pass "relative entry with CDPATH set"; else fail "relative entry with CDPATH set"; fi

echo "install-path: the suggested export line can be pasted"
run_check "$other:$SYS" "$install"
line=$(printf '%s\n' "$OUT" | grep 'export PATH=' | head -n 1)
if [ -n "$line" ] && /bin/sh -n -c "$line" 2>/dev/null; then
    pass "export line parses as shell"
else
    fail "export line parses as shell"
fi

echo "install-path: the suggested export line adds the exact directory"
# Run the printed command and read PATH back: the first entry must be the
# install directory byte for byte, whatever characters its name holds.
for name in 'plain' 'with$USER' 'with"quote' "with'single" 'with space' 'with`tick`' 'with\back'; do
    dir="$WORK/odd/$name/bin"
    mkdir -p "$dir"
    fake_bin "$dir"
    run_check "$other:$SYS" "$dir"
    line=$(printf '%s\n' "$OUT" | grep 'export PATH=' | head -n 1 | sed 's/^ *//')
    got=$(PATH=/usr/bin:/bin USER=expanded /bin/sh -c "$line"'; printf "%s" "$PATH"' 2>/dev/null | sed 's/:.*//')
    if [ "$got" = "$dir" ]; then pass "export adds the exact directory: $name"; else fail "export adds the exact directory: $name (got '$got')"; fi
done

if [ "$FAIL" -ne 0 ]; then
    echo "install-path tests FAILED"
    exit 1
fi
echo "install-path tests passed"
