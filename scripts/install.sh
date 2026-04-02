#!/bin/bash
#
# agent-harness installer script
# Usage: curl -fsSL https://raw.githubusercontent.com/BA-CalderonMorales/agent-harness/main/scripts/install.sh | bash
#

set -e

# Configuration
REPO="BA-CalderonMorales/agent-harness"
BINARY_NAME="agent-harness"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Detect OS and architecture
detect_platform() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)
    
    case "$os" in
        linux)
            OS="linux"
            ;;
        darwin)
            OS="darwin"
            ;;
        mingw*|cygwin*|msys*)
            OS="windows"
            ;;
        *)
            log_error "Unsupported operating system: $os"
            exit 1
            ;;
    esac
    
    case "$arch" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        arm64|aarch64)
            ARCH="arm64"
            ;;
        armv7l|armv6l)
            ARCH="arm"
            ;;
        i386|i686)
            ARCH="386"
            ;;
        *)
            log_error "Unsupported architecture: $arch"
            exit 1
            ;;
    esac
    
    log_info "Detected platform: $OS/$ARCH"
}

# Get latest release version
get_latest_version() {
    log_info "Fetching latest release..."
    
    local latest_url="https://api.github.com/repos/$REPO/releases/latest"
    VERSION=$(curl -fsSL "$latest_url" | grep -oP '"tag_name": "\K[^"]+' || true)
    
    if [ -z "$VERSION" ]; then
        log_error "Failed to fetch latest version"
        exit 1
    fi
    
    log_info "Latest version: $VERSION"
}

# Download and install binary
download_binary() {
    local suffix="${OS}-${ARCH}"
    local archive_name="${BINARY_NAME}-${suffix}"
    
    if [ "$OS" = "windows" ]; then
        archive_name="${archive_name}.zip"
    else
        archive_name="${archive_name}.tar.gz"
    fi
    
    local download_url="https://github.com/$REPO/releases/download/$VERSION/$archive_name"
    local temp_dir=$(mktemp -d)
    
    log_info "Downloading $archive_name..."
    
    if ! curl -fsSL -o "$temp_dir/$archive_name" "$download_url"; then
        log_error "Failed to download from: $download_url"
        rm -rf "$temp_dir"
        exit 1
    fi
    
    log_info "Extracting archive..."
    
    cd "$temp_dir"
    
    if [ "$OS" = "windows" ]; then
        unzip -q "$archive_name"
        binary_name="${BINARY_NAME}-${suffix}"
    else
        tar -xzf "$archive_name"
        binary_name="${BINARY_NAME}-${suffix}"
    fi
    
    # Set binary name for Windows
    if [ "$OS" = "windows" ]; then
        BINARY_NAME="${BINARY_NAME}.exe"
        binary_name="${binary_name}.exe"
    fi
    
    log_info "Installing to $INSTALL_DIR..."
    
    # Check if we need sudo
    if [ -w "$INSTALL_DIR" ]; then
        mv "$binary_name" "$INSTALL_DIR/$BINARY_NAME"
        chmod +x "$INSTALL_DIR/$BINARY_NAME"
    else
        log_warn "Root privileges required for installation to $INSTALL_DIR"
        sudo mv "$binary_name" "$INSTALL_DIR/$BINARY_NAME"
        sudo chmod +x "$INSTALL_DIR/$BINARY_NAME"
    fi
    
    # Cleanup
    cd -
    rm -rf "$temp_dir"
    
    log_success "Installed $BINARY_NAME $VERSION to $INSTALL_DIR"
}

# Verify installation
verify_installation() {
    if command -v "$BINARY_NAME" &> /dev/null; then
        log_success "Installation verified!"
        echo ""
        "$BINARY_NAME" --version 2>/dev/null || true
        echo ""
        log_info "Run '$BINARY_NAME' to get started"
    else
        log_warn "$BINARY_NAME not found in PATH"
        log_info "You may need to restart your shell or run: export PATH=\"$INSTALL_DIR:\$PATH\""
    fi
}

# Main installation flow
main() {
    echo ""
    echo "╔════════════════════════════════════════════════════════════╗"
    echo "║     Agent Harness Installer                                ║"
    echo "╚════════════════════════════════════════════════════════════╝"
    echo ""
    
    # Handle flags
    while [[ $# -gt 0 ]]; do
        case $1 in
            --version|-v)
                VERSION="$2"
                shift 2
                ;;
            --dir|-d)
                INSTALL_DIR="$2"
                shift 2
                ;;
            --help|-h)
                echo "Usage: $0 [OPTIONS]"
                echo ""
                echo "Options:"
                echo "  --version, -v    Install specific version (default: latest)"
                echo "  --dir, -d        Installation directory (default: /usr/local/bin)"
                echo "  --help, -h       Show this help message"
                echo ""
                echo "Environment variables:"
                echo "  INSTALL_DIR      Installation directory"
                echo "  GITHUB_TOKEN     GitHub token for API requests (optional)"
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                exit 1
                ;;
        esac
    done
    
    # Check dependencies
    if ! command -v curl &> /dev/null; then
        log_error "curl is required but not installed"
        exit 1
    fi
    
    # Detect platform
    detect_platform
    
    # Get version
    if [ -z "$VERSION" ]; then
        get_latest_version
    fi
    
    # Download and install
    download_binary
    
    # Verify
    verify_installation
    
    echo ""
    echo "╔════════════════════════════════════════════════════════════╗"
    echo "║  Installation Complete!                                    ║"
    echo "╚════════════════════════════════════════════════════════════╝"
    echo ""
}

main "$@"
