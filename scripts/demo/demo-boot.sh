#!/usr/bin/env bash
# Boot agent-harness straight into the TUI for demo recordings.
#
# Forces the local provider against the mock LLM server (scripts/demo/
# mock-llm.py) so the app skips the first-run setup wizard and lands
# directly on the Home tab. Record with the mock already running:
#   python3 scripts/demo/mock-llm.py   (terminal 1)
#   vhs scripts/demo/tui.tape          (terminal 2)
#
# Real creds never touch this path: env vars override every config
# layer and nothing is persisted.
set -euo pipefail
export AH_PROVIDER=local
export AH_RUNTIME=llama.cpp
export AH_MODEL=demo-1.0
export AH_ENDPOINT_URL=http://127.0.0.1:8080/v1
export AH_API_KEY=local
# The tool-burst demo executes real (harmless) bash calls; the fresh
# config defaults to read-only, so the demo pins workspace-write and
# execute for the collapse to have anything to collapse.
export AH_PERMISSION_MODE=workspace-write
export AH_PERM_EXECUTE=true
exec ./build/agent-harness "$@"
