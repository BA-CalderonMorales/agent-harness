#!/data/data/com.termux/files/usr/bin/bash
#
# agent-harness installer for Termux
# Builds from source since static binaries don't work on Termux
#

set -e

repo="ba-calderonmorales/agent-harness"
binary_name="agent-harness"
install_dir="${install_dir:-$prefix/bin}"

# colors
red='\033[0;31m'
green='\033[0;32m'
yellow='\033[1;33m'
blue='\033[0;34m'
nc='\033[0m'

log_info() { echo -e "${blue}[info]${nc} $1"; }
log_success() { echo -e "${green}[success]${nc} $1"; }
log_warn() { echo -e "${yellow}[warn]${nc} $1"; }
log_error() { echo -e "${red}[error]${nc} $1"; }

# check dependencies
check_deps() {
    if ! command -v go &> /dev/null; then
        log_error "go is not installed. install it with: pkg install golang"
        exit 1
    fi
    if ! command -v git &> /dev/null; then
        log_error "git is not installed. install it with: pkg install git"
        exit 1
    fi
}

# get latest version from github
get_latest_version() {
    log_info "fetching latest release..."
    version=$(curl -fssl "https://api.github.com/repos/$repo/releases/latest" 2>/dev/null | grep -o '"tag_name": "[^"]*"' | cut -d'"' -f4 || echo "")
    if [ -z "$version" ]; then
        log_warn "could not fetch version, using main branch"
        version="main"
    fi
    log_info "version: $version"
}

# build from source
build_from_source() {
    local temp=$(mktemp -d)
    local version="${1:-main}"
    
    log_info "cloning repository..."
    git clone --depth 1 --branch "$version" "https://github.com/$repo.git" "$temp" 2>/dev/null || {
        log_warn "failed to clone tag $version, trying main..."
        git clone --depth 1 "https://github.com/$repo.git" "$temp"
    }
    
    log_info "building agent-harness..."
    cd "$temp"
    go build -ldflags "-s -w -X main.Version=$version" -o "$temp/$binary_name" ./cmd/agent-harness
    
    log_info "installing to $install_dir..."
    mv "$temp/$binary_name" "$install_dir/$binary_name"
    chmod +x "$install_dir/$binary_name"
    
    rm -rf "$temp"
    log_success "installed $binary_name $version"
}

# quick build from local source (if in repo)
build_local() {
    local repo_root="$home/projects/agent-harness"
    
    if [ -d "$repo_root/.git" ]; then
        log_info "building from local source..."
        cd "$repo_root"
        go build -ldflags "-s -w -X main.Version=local" -o "$install_dir/$binary_name" ./cmd/agent-harness
        chmod +x "$install_dir/$binary_name"
        log_success "installed $binary_name from local source"
        return 0
    fi
    return 1
}

setup_shell_integration() {
    log_info "setting up shell integration..."
    
    # create wrapper function for 'ah' alias
    local shell_rc=""
    if [ -n "$bash_version" ]; then
        shell_rc="$home/.bashrc"
    elif [ -n "$zsh_version" ]; then
        shell_rc="$home/.zshrc"
    fi
    
    if [ -n "$shell_rc" ] && [ -f "$shell_rc" ]; then
        # check if already added
        if ! grep -q "agent-harness workspace" "$shell_rc" 2>/dev/null; then
            cat >> "$shell_rc" << 'eof'

# agent harness workspace shortcut
ah() {
    if [ $# -eq 0 ]; then
        agent-harness
    else
        agent-harness "$@"
    fi
}
eof
            log_success "added 'ah' function to $shell_rc"
            log_info "run 'source $shell_rc' to apply changes"
        fi
    fi
}

main() {
    echo ""
    echo "╔════════════════════════════════════════════════════════════╗"
    echo "║     agent harness installer (termux)                       ║"
    echo "╚════════════════════════════════════════════════════════════╝"
    echo ""
    
    check_deps
    
    # if running from within the repo, do a quick local build
    if build_local 2>/dev/null; then
        setup_shell_integration
        echo ""
        echo "installation complete!"
        echo ""
        echo "usage:"
        echo "  agent-harness        start interactive mode"
        echo "  ah                   short alias (after shell reload)"
        echo ""
        exit 0
    fi
    
    # otherwise build from github
    get_latest_version
    build_from_source "$version"
    setup_shell_integration
    
    echo ""
    echo "installation complete!"
    echo ""
    echo "usage:"
    echo "  agent-harness        start interactive mode"
    echo "  ah                   short alias (after shell reload)"
    echo ""
}

main "$@"
