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
    if resolved=$(cd -P -- "$1" 2>/dev/null && pwd -P); then
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

if [ "$on_path" -eq 0 ]; then
    {
        printf '\n[warn] %s is not on your PATH, so the %s just installed will not run by name.\n' "$want" "$name"
        if [ -n "$first" ] && ! same_binary "$first" "$installed"; then
            printf '[warn] Your shell will run %s instead.\n' "$first"
        fi
        printf '       Fix it one of two ways:\n'
        printf '         1. Install into a directory already on PATH:\n'
        printf '              GOBIN=$HOME/.local/bin make install\n'
        printf '         2. Add %s to PATH in your shell profile:\n' "$want"
        printf '              export PATH="%s:$PATH"\n' "$want"
    } >&2
    exit 0
fi

if [ -n "$first" ] && ! same_binary "$first" "$installed"; then
    {
        printf '\n[warn] %s comes earlier on your PATH and shadows the %s just installed.\n' "$first" "$name"
        printf '       Installed: %s\n' "$installed"
        printf '       Running %s will start %s.\n' "$name" "$first"
        printf '       Remove the old binary, or move %s ahead of %s on PATH.\n' "$want" "${first%/*}"
    } >&2
fi
exit 0
