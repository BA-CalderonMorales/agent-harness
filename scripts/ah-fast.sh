#!/data/data/com.termux/files/usr/bin/env bash
#
# ah-fast: Run agent-harness with local Ollama LLM (Gemma 2B for speed)
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BUILD_DIR="$PROJECT_ROOT/build"
BINARY="$BUILD_DIR/agent-harness"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() { echo -e "${GREEN}[ah-fast]${NC} $1"; }
warn() { echo -e "${YELLOW}[ah-fast]${NC} $1"; }
error() { echo -e "${RED}[ah-fast]${NC} $1"; }
info() { echo -e "${BLUE}[ah-fast]${NC} $1"; }

# Help
if [[ "$1" == "--help" || "$1" == "-h" ]]; then
    echo "ah-fast: Agent Harness with local Ollama (Gemma 2B - fast)"
    echo ""
    echo "Usage: ah-fast [options]"
    echo ""
    echo "Options:"
    echo "  -h, --help     Show this help"
    echo ""
    echo "Environment:"
    echo "  AH_FAST_MODEL     Override default model (default: gemma4:2b)"
    echo "  OLLAMA_HOST       Ollama server URL (default: http://localhost:11434)"
    echo ""
    echo "Examples:"
    echo "  ah-fast           # Run with gemma4:2b (fast)"
    exit 0
fi

# Check if Ollama is running
OLLAMA_HOST="${OLLAMA_HOST:-http://localhost:11434}"
if ! curl -s "$OLLAMA_HOST/api/tags" > /dev/null 2>&1; then
    error "Ollama is not running at $OLLAMA_HOST"
    error "Start it with: ~/start_ollama.sh"
    exit 1
fi

# Set model - 2B for speed
FAST_MODEL="${AH_FAST_MODEL:-gemma4:2b}"

# Check if model is available
if ! curl -s "$OLLAMA_HOST/api/tags" | grep -q "\"name\":\"$FAST_MODEL\""; then
    warn "Model '$FAST_MODEL' not found locally"
    info "Pulling model (this may take a while)..."
    ollama pull "$FAST_MODEL"
fi

log "Using fast local model: $FAST_MODEL"

# Build if needed
if [[ ! -f "$BINARY" ]] || [[ "$PROJECT_ROOT/cmd/agent-harness/main.go" -nt "$BINARY" ]]; then
    log "Building agent-harness..."
    cd "$PROJECT_ROOT"
    go build -ldflags "-X main.Version=fast-dev" -o "$BINARY" ./cmd/agent-harness
    log "Build complete"
fi

# Run with local provider
export AH_PROVIDER="ollama"
export AH_MODEL="$FAST_MODEL"
export AH_API_KEY="ollama"  # Required but not used for local

log "Starting agent-harness with fast local LLM..."
cd "$PROJECT_ROOT"
exec "$BINARY"
