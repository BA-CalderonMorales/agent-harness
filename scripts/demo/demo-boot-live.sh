#!/usr/bin/env bash
# Boot agent-harness for the live demo recording: the real deal — a
# hosted GLM model through the fireworks provider, running a real turn
# with tool calls and markdown output.
#
# The fireworks API key comes from the encrypted credential store
# (already configured via /login). Sessions/logs go to a dedicated
# scratch data dir so every recording starts with a clean transcript:
#
#   vhs scripts/demo/tui.tape
#
# Env vars override every config layer and nothing is persisted.
set -euo pipefail
export AH_PROVIDER=fireworks
export AH_MODEL=accounts/fireworks/models/glm-5p3-flash
export AH_PERMISSION_MODE=workspace-write
export AH_PERM_EXECUTE=true
DEMO_DATA="${TMPDIR:-/tmp}/agent-harness-demo-data"
rm -rf "$DEMO_DATA"
mkdir -p "$DEMO_DATA"
export XDG_DATA_HOME="$DEMO_DATA"
exec ./build/agent-harness "$@"
