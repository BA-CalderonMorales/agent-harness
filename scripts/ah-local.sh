#!/usr/bin/env bash
#
# ah-local: run agent-harness against a local OpenAI-compatible server.
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BUILD_DIR="$PROJECT_ROOT/build"
BINARY="$BUILD_DIR/agent-harness"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() { printf "%b[ah-local]%b %s\n" "$GREEN" "$NC" "$1"; }
warn() { printf "%b[ah-local]%b %s\n" "$YELLOW" "$NC" "$1"; }
error() { printf "%b[ah-local]%b %s\n" "$RED" "$NC" "$1"; }
info() { printf "%b[ah-local]%b %s\n" "$BLUE" "$NC" "$1"; }

if [[ "$1" == "--help" || "$1" == "-h" ]]; then
    echo "ah-local: Agent Harness with a local OpenAI-compatible server"
    echo ""
    echo "Usage: ah-local"
    echo ""
    echo "Environment:"
    echo "  AH_MODEL                  Model name (default: deepreinforce-ai/Ornith-1.0-9B-GGUF)"
    echo "  AH_ENDPOINT_URL           API base URL (default: http://127.0.0.1:8080/v1)"
    echo "  AH_CONTEXT_LENGTH         Context window (default: 8192)"
    echo "  AH_MAX_TOKENS             Response token limit (default: 4096)"
    echo "  AH_LOCAL_SERVER_COMMAND   Command hint shown when the server is not reachable"
    echo ""
    echo "Example server:"
    echo "  llama-server -m ./models/ornith-1.0-9b-Q4_K_M.gguf -c 8192 --host 127.0.0.1 --port 8080"
    exit 0
fi

LOCAL_ENDPOINT="${AH_ENDPOINT_URL:-http://127.0.0.1:8080/v1}"
SERVER_COMMAND="${AH_LOCAL_SERVER_COMMAND:-llama-server -m ./models/ornith-1.0-9b-Q4_K_M.gguf -c 8192 --host 127.0.0.1 --port 8080}"

if ! curl -fsS "$LOCAL_ENDPOINT/models" >/dev/null 2>&1; then
    warn "No local OpenAI-compatible server responded at $LOCAL_ENDPOINT"
    info "Start one with:"
    info "  $SERVER_COMMAND"
    exit 1
fi

if [[ ! -f "$BINARY" ]] || [[ "$PROJECT_ROOT/cmd/agent-harness/main.go" -nt "$BINARY" ]]; then
    log "Building agent-harness"
    cd "$PROJECT_ROOT"
    go build -ldflags "-X main.Version=local-dev" -o "$BINARY" ./cmd/agent-harness
fi

export AH_PROVIDER="${AH_PROVIDER:-local}"
export AH_RUNTIME="${AH_RUNTIME:-llama.cpp}"
export AH_MODEL="${AH_MODEL:-deepreinforce-ai/Ornith-1.0-9B-GGUF}"
export AH_ENDPOINT_URL="$LOCAL_ENDPOINT"
export AH_CONTEXT_LENGTH="${AH_CONTEXT_LENGTH:-8192}"
export AH_MAX_TOKENS="${AH_MAX_TOKENS:-4096}"
export AH_API_KEY="${AH_API_KEY:-local}"

log "Starting agent-harness with $AH_MODEL at $AH_ENDPOINT_URL"
cd "$PROJECT_ROOT"
exec "$BINARY"
