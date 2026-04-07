#!/data/data/com.termux/files/usr/bin/env bash
#
# ah-local: Run agent-harness with local Ollama LLM (Gemma 4B for quality)
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

log() { echo -e "${GREEN}[ah-local]${NC} $1"; }
warn() { echo -e "${YELLOW}[ah-local]${NC} $1"; }
error() { echo -e "${RED}[ah-local]${NC} $1"; }
info() { echo -e "${BLUE}[ah-local]${NC} $1"; }

# Help
if [[ "$1" == "--help" || "$1" == "-h" ]]; then
    echo "ah-local: Agent Harness with local Ollama (Gemma 4B)"
    echo ""
    echo "Usage: ah-local [options]"
    echo ""
    echo "Options:"
    echo "  -h, --help     Show this help"
    echo "  --model MODEL  Use specific Ollama model (default: gemma4:4b)"
    echo ""
    echo "Environment:"
    echo "  AH_LOCAL_MODEL    Override default model"
    echo "  OLLAMA_HOST       Ollama server URL (default: http://localhost:11434)"
    echo ""
    echo "Examples:"
    echo "  ah-local                    # Run with gemma4:4b"
    echo "  ah-local --model gemma4:2b  # Run with faster 2B model"
    exit 0
fi

# Check if Ollama is running
OLLAMA_HOST="${OLLAMA_HOST:-http://localhost:11434}"
if ! curl -s "$OLLAMA_HOST/api/tags" > /dev/null 2>&1; then
    error "Ollama is not running at $OLLAMA_HOST"
    error "Start it with: ~/start_ollama.sh"
    exit 1
fi

# Set model
LOCAL_MODEL="${AH_LOCAL_MODEL:-gemma4:4b}"
if [[ "$1" == "--model" && -n "$2" ]]; then
    LOCAL_MODEL="$2"
    shift 2
fi

# Check if model is available
if ! curl -s "$OLLAMA_HOST/api/tags" | grep -q "\"name\":\"$LOCAL_MODEL\""; then
    warn "Model '$LOCAL_MODEL' not found locally"
    info "Pulling model (this may take a while)..."
    ollama pull "$LOCAL_MODEL"
fi

log "Using local model: $LOCAL_MODEL"

# Build if needed
if [[ ! -f "$BINARY" ]] || [[ "$PROJECT_ROOT/cmd/agent-harness/main.go" -nt "$BINARY" ]]; then
    log "Building agent-harness..."
    cd "$PROJECT_ROOT"
    go build -ldflags "-X main.Version=local-dev" -o "$BINARY" ./cmd/agent-harness
    log "Build complete"
fi

# Run with local provider
export AH_PROVIDER="ollama"
export AH_MODEL="$LOCAL_MODEL"
export AH_API_KEY="ollama"  # Required but not used for local

log "Starting agent-harness with local LLM..."
cd "$PROJECT_ROOT"
exec "$BINARY"
