#!/usr/bin/env bash
# Build and launch with the verified Lemonade profile in the caller's workspace.
set -euo pipefail

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  cat <<'HELP'
Usage: bash /path/to/agent-harness/scripts/ah-lemonade.sh [app flags]
Run from the workspace you want the agent to use.

Overrides: AH_LEMONADE_ENDPOINT, AH_LEMONADE_MODEL, AH_LEMONADE_API_KEY,
           AH_LEMONADE_CONTEXT_LENGTH, AH_LEMONADE_MAX_TOKENS
Requires a running Lemonade server and an available model; installs nothing.
HELP
  exit 0
fi

project_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
binary="$project_root/build/agent-harness"

export AH_PROVIDER=local
export AH_RUNTIME=lemonade
export AH_ENDPOINT_URL="${AH_LEMONADE_ENDPOINT:-http://127.0.0.1:13305/v1}"
export AH_MODEL="${AH_LEMONADE_MODEL:-qwen3.5-9b-FLM}"
# Never inherit a hosted provider's API key for this local profile.
export AH_API_KEY="${AH_LEMONADE_API_KEY:-lemonade-local}"
export AH_CONTEXT_LENGTH="${AH_LEMONADE_CONTEXT_LENGTH:-8192}"
export AH_MAX_TOKENS="${AH_LEMONADE_MAX_TOKENS:-4096}"

python - <<'PY'
import json
import os
import sys
import urllib.error
import urllib.request

endpoint = os.environ['AH_ENDPOINT_URL'].rstrip('/')
model = os.environ['AH_MODEL']
request = urllib.request.Request(endpoint + '/models', headers={
    'Authorization': 'Bearer ' + os.environ['AH_API_KEY'],
})
try:
    with urllib.request.urlopen(request, timeout=5) as response:
        models = json.load(response)
    if not any(item.get('id') == model for item in models.get('data', [])):
        sys.exit(f'Model {model!r} is not listed at {endpoint}. Check lemonade list/status.')
except (OSError, ValueError, AttributeError) as error:
    # Avoid printing response bodies or headers, which may contain credentials.
    sys.exit(f'Lemonade preflight failed ({type(error).__name__}). Check lemonade status and endpoint.')
PY

mkdir -p -- "$project_root/build"
go -C "$project_root" build -o "$binary" ./cmd/agent-harness
printf 'Using %s via Lemonade in %s\n' "$AH_MODEL" "$PWD"
exec "$binary" "$@"
