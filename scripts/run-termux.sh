#!/data/data/com.termux/files/usr/bin/env bash
#
# Run agent-harness in Termux environment
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
NC='\033[0m'

log() { echo -e "${GREEN}[termux-run]${NC} $1"; }
warn() { echo -e "${YELLOW}[termux-run]${NC} $1"; }
error() { echo -e "${RED}[termux-run]${NC} $1"; }

# Check API key
if [[ -z "$OPENROUTER_API_KEY" && -z "$ANTHROPIC_API_KEY" ]]; then
    error "No API key found! Set OPENROUTER_API_KEY or ANTHROPIC_API_KEY"
    exit 1
fi

# Build if needed
if [[ ! -f "$BINARY" ]] || [[ "$PROJECT_ROOT/cmd/agent-harness/main.go" -nt "$BINARY" ]]; then
    log "Building agent-harness..."
    cd "$PROJECT_ROOT"
    go build -ldflags "-X main.Version=termux-dev" -o "$BINARY" ./cmd/agent-harness
    log "Build complete"
fi

# Detect TUI mode
USE_TUI=""
if [[ "$1" == "--tui" ]]; then
    USE_TUI="--tui"
    warn "TUI mode may have rendering issues on mobile keyboards"
fi

# Run
log "Starting agent-harness..."
cd "$PROJECT_ROOT"
exec "$BINARY" $USE_TUI
