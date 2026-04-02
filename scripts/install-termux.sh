#!/data/data/com.termux/files/usr/bin/bash
#
# agent-harness installer for Termux
# Optimized for Android/Termux environment
#

set -e

REPO="BA-CalderonMorales/agent-harness"
BINARY_NAME="agent-harness"
INSTALL_DIR="${INSTALL_DIR:-$PREFIX/bin}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Detect architecture (Termux uses different naming)
detect_arch() {
    local arch=$(uname -m)
    case "$arch" in
        aarch64) ARCH="arm64" ;;
        armv7l|armv8l) ARCH="arm64" ;;
        x86_64) ARCH="amd64" ;;
        i386|i686) ARCH="386" ;;
        *) log_error "Unsupported architecture: $arch"; exit 1 ;;
    esac
    log_info "Detected architecture: $ARCH"
}

get_latest_version() {
    log_info "Fetching latest release..."
    VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep -oP '"tag_name": "\K[^"]+' || true)
    [ -z "$VERSION" ] && { log_error "Failed to fetch version"; exit 1; }
    log_info "Latest version: $VERSION"
}

download_and_install() {
    local archive="${BINARY_NAME}-linux-${ARCH}.tar.gz"
    local url="https://github.com/$REPO/releases/download/$VERSION/$archive"
    local temp=$(mktemp -d)
    
    log_info "Downloading $archive..."
    curl -fsSL -o "$temp/$archive" "$url" || {
        log_error "Download failed: $url"
        rm -rf "$temp"
        exit 1
    }
    
    log_info "Extracting..."
    tar -xzf "$temp/$archive" -C "$temp"
    
    log_info "Installing to $INSTALL_DIR..."
    mv "$temp/${BINARY_NAME}-linux-${ARCH}" "$INSTALL_DIR/$BINARY_NAME"
    chmod +x "$INSTALL_DIR/$BINARY_NAME"
    
    rm -rf "$temp"
    log_success "Installed $BINARY_NAME $VERSION"
}

setup_shell_integration() {
    log_info "Setting up shell integration..."
    
    # Create wrapper function for 'ah' alias
    local shell_rc=""
    if [ -n "$BASH_VERSION" ]; then
        shell_rc="$HOME/.bashrc"
    elif [ -n "$ZSH_VERSION" ]; then
        shell_rc="$HOME/.zshrc"
    fi
    
    if [ -n "$shell_rc" ] && [ -f "$shell_rc" ]; then
        # Check if already added
        if ! grep -q "agent-harness workspace" "$shell_rc" 2>/dev/null; then
            cat >> "$shell_rc" << 'EOF'

# Agent Harness workspace shortcut
ah() {
    if [ $# -eq 0 ]; then
        agent-harness
    else
        agent-harness "$@"
    fi
}
EOF
            log_success "Added 'ah' function to $shell_rc"
            log_info "Run 'source $shell_rc' to apply changes"
        fi
    fi
}

main() {
    echo ""
    echo "╔════════════════════════════════════════════════════════════╗"
    echo "║     Agent Harness Installer (Termux)                       ║"
    echo "╚════════════════════════════════════════════════════════════╝"
    echo ""
    
    detect_arch
    [ -z "$VERSION" ] && get_latest_version
    download_and_install
    setup_shell_integration
    
    echo ""
    echo "Installation complete!"
    echo ""
    echo "Usage:"
    echo "  agent-harness        Start interactive mode"
    echo "  ah                   Short alias (after shell reload)"
    echo ""
}

main "$@"
