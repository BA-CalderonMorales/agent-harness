#!/bin/sh
# Exercise substring collisions using a fake package inventory.
set -eu
repo_root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT HUP INT TERM
cat > "$fixture/go" <<'MOCK'
#!/bin/sh
case "$2" in
./internal/interface/tui/...) printf '%s\n' example/internal/interface/tui ;;
./internal/core/...) printf '%s\n' example/internal/core/state ;;
./internal/agent/...) printf '%s\n' example/internal/agent ;;
./...) printf '%s\n' example/internal/interface example/internal/interface/tui example/internal/core example/internal/core/state example/internal/agent example/internal/agentextra ;;
*) exit 1 ;;
esac
MOCK
chmod +x "$fixture/go"
actual=$(PATH="$fixture:$PATH" sh "$repo_root/scripts/test-shards.sh" list rest)
expected=$(printf '%s\n' example/internal/interface example/internal/core example/internal/agentextra)
[ "$actual" = "$expected" ] || { printf 'Incorrect rest shard:\n%s\n' "$actual"; exit 1; }
PATH="$fixture:$PATH" sh "$repo_root/scripts/test-shards.sh" coverage
