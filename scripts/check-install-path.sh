#!/bin/sh
# check-install-path.sh - warn when an installed binary is not the one
# the shell will run.
#
# Usage: check-install-path.sh <bindir> <binary-name>
#
# Silent when <bindir> is on PATH and the first <binary-name> on PATH is
# <bindir>/<binary-name>. Otherwise it prints a [warn] to stderr. It
# always exits 0: a misplaced install is worth a warning, not a failure.
#
# Only shell builtins are used, so it behaves the same on GNU and BSD
# systems and still works when PATH itself is broken.

if [ "$#" -ne 2 ]; then
    printf 'usage: %s <bindir> <binary-name>\n' "$0" >&2
    exit 0
fi

bindir=$1
name=$2

# canon DIR prints DIR with symlinks and trailing slashes resolved. A
# directory that does not exist is printed with trailing slashes removed.
canon() {
    # CDPATH is cleared so cd never searches elsewhere or prints the
    # directory it found, which would corrupt a relative entry.
    if resolved=$(CDPATH= cd -P -- "$1" 2>/dev/null && pwd -P); then
        printf '%s\n' "$resolved"
        return
    fi
    d=$1
    while :; do
        case $d in
            /) break ;;
            */) d=${d%/} ;;
            *) break ;;
        esac
    done
    printf '%s\n' "$d"
}

want=$(canon "$bindir")
installed="$want/$name"

on_path=0
first=""
rest=$PATH
while :; do
    case $rest in
        *:*) entry=${rest%%:*}; rest=${rest#*:}; more=1 ;;
        *) entry=$rest; more=0 ;;
    esac
    # POSIX: an empty PATH entry means the current directory.
    [ -n "$entry" ] || entry=.
    dir=$(canon "$entry")
    [ "$dir" = "$want" ] && on_path=1
    if [ -z "$first" ] && [ -f "$dir/$name" ] && [ -x "$dir/$name" ]; then
        first="$dir/$name"
    fi
    [ "$more" -eq 1 ] || break
done
# An empty PATH has no entries at all, not one empty entry.
if [ -z "$PATH" ]; then
    on_path=0
    first=""
fi

same_binary() {
    [ "$1" = "$2" ] && return 0
    # -ef is not in POSIX test, but dash, bash, ksh, zsh, busybox, and
    # BSD sh all support it; it catches a symlink to the installed file.
    [ "$1" -ef "$2" ] 2>/dev/null
}

# is_shim FILE succeeds when FILE is a version-manager shim: it lives in
# a directory named shims, or it is a symlink to a differently named
# program (mise, asdf). Plain readlink, when present, behaves the same
# on GNU and BSD; without it only the directory name is checked.
is_shim() {
    case ${1%/*} in */shims) return 0 ;; esac
    [ -L "$1" ] || return 1
    command -v readlink >/dev/null 2>&1 || return 1
    target=$(readlink "$1" 2>/dev/null) || return 1
    [ "${target##*/}" != "${1##*/}" ]
}

shadowed=0
if [ -n "$first" ] && ! same_binary "$first" "$installed"; then
    shadowed=1
fi
[ "$on_path" -eq 0 ] || [ "$shadowed" -eq 1 ] || exit 0

# The advice only ever names $want first on PATH, the one placement
# that is sure to win; a fixed directory can itself be shadowed.
{
    printf '\n'
    if [ "$on_path" -eq 0 ]; then
        printf '[warn] %s is not on your PATH, so the %s just installed will not run by name.\n' "$want" "$name"
    fi
    if [ "$shadowed" -eq 1 ]; then
        printf '[warn] Running %s starts %s, not %s.\n' "$name" "$first" "$installed"
        if is_shim "$first"; then
            printf '       %s is a shim from a version manager; it will keep running its own copy.\n' "$first"
            printf '       Fix: put %s ahead of %s on PATH:\n' "$want" "${first%/*}"
        else
            printf '       Fix: remove or replace %s, or put %s ahead of %s on PATH:\n' "$first" "$want" "${first%/*}"
        fi
    else
        printf '       Fix: add %s to PATH:\n' "$want"
    fi
    printf '         export PATH="%s:$PATH"\n' "$want"
    printf '         # Add that export line to your shell profile to keep it.\n'
} >&2
exit 0
