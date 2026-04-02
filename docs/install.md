# Installation Guide

Complete installation instructions for agent-harness on all supported platforms.

---

## Table of Contents

- [Quick Install](#quick-install)
- [Linux](#linux)
- [macOS](#macos)
- [Windows](#windows)
- [Termux (Android)](#termux-android)
- [Docker](#docker)
- [Build from Source](#build-from-source)
- [Post-Installation](#post-installation)
- [Troubleshooting](#troubleshooting)

---

## Quick Install

### Universal Installer (Linux/macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/BA-CalderonMorales/agent-harness/main/scripts/install.sh | bash
```

### Termux (Android)

```bash
curl -fsSL https://raw.githubusercontent.com/BA-CalderonMorales/agent-harness/main/scripts/install-termux.sh | bash
```

---

## Linux

### AMD64 (x86_64)

```bash
# Download latest release
curl -L -o agent-harness.tar.gz \
  https://github.com/BA-CalderonMorales/agent-harness/releases/latest/download/agent-harness-linux-amd64.tar.gz

# Extract
tar -xzf agent-harness.tar.gz

# Install to /usr/local/bin
sudo mv agent-harness-linux-amd64 /usr/local/bin/agent-harness
sudo chmod +x /usr/local/bin/agent-harness

# Or install to ~/.local/bin
mkdir -p ~/.local/bin
mv agent-harness-linux-amd64 ~/.local/bin/agent-harness
chmod +x ~/.local/bin/agent-harness
```

### ARM64 (aarch64)

```bash
# Download latest release
curl -L -o agent-harness.tar.gz \
  https://github.com/BA-CalderonMorales/agent-harness/releases/latest/download/agent-harness-linux-arm64.tar.gz

# Extract and install
tar -xzf agent-harness.tar.gz
sudo mv agent-harness-linux-arm64 /usr/local/bin/agent-harness
sudo chmod +x /usr/local/bin/agent-harness
```

### Package Managers

**Coming soon:**
- AUR (Arch Linux)
- Homebrew (Linux)
- Snap

---

## macOS

### Intel (AMD64)

```bash
# Download latest release
curl -L -o agent-harness.tar.gz \
  https://github.com/BA-CalderonMorales/agent-harness/releases/latest/download/agent-harness-darwin-amd64.tar.gz

# Extract and install
tar -xzf agent-harness.tar.gz
sudo mv agent-harness-darwin-amd64 /usr/local/bin/agent-harness
sudo chmod +x /usr/local/bin/agent-harness
```

### Apple Silicon (ARM64)

```bash
# Download latest release
curl -L -o agent-harness.tar.gz \
  https://github.com/BA-CalderonMorales/agent-harness/releases/latest/download/agent-harness-darwin-arm64.tar.gz

# Extract and install
tar -xzf agent-harness.tar.gz
sudo mv agent-harness-darwin-arm64 /usr/local/bin/agent-harness
sudo chmod +x /usr/local/bin/agent-harness
```

### Homebrew (Coming Soon)

```bash
# Will be available once Homebrew formula is accepted
brew install agent-harness
```

---

## Windows

### PowerShell (Recommended)

```powershell
# Download latest release
Invoke-WebRequest -Uri "https://github.com/BA-CalderonMorales/agent-harness/releases/latest/download/agent-harness-windows-amd64.exe.zip" -OutFile "agent-harness.zip"

# Extract
Expand-Archive -Path "agent-harness.zip" -DestinationPath "$env:USERPROFILE\bin"

# Add to PATH (if not already done)
[Environment]::SetEnvironmentVariable("Path", "$env:Path;$env:USERPROFILE\bin", "User")
```

### Manual Install

1. Download the appropriate `.zip` file from [releases](https://github.com/BA-CalderonMorales/agent-harness/releases)
2. Extract to a folder (e.g., `C:\Program Files\agent-harness`)
3. Add the folder to your PATH environment variable

---

## Termux (Android)

Termux requires special handling due to the Android environment.

### Using the Termux Installer

```bash
curl -fsSL https://raw.githubusercontent.com/BA-CalderonMorales/agent-harness/main/scripts/install-termux.sh | bash
```

### Manual Install

```bash
# Update packages
pkg update

# Install required packages
pkg install curl git

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
    aarch64) SUFFIX="linux-arm64" ;;
    armv7l|armv8l) SUFFIX="linux-arm64" ;;
    x86_64) SUFFIX="linux-amd64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Download and install
curl -L -o agent-harness.tar.gz \
  "https://github.com/BA-CalderonMorales/agent-harness/releases/latest/download/agent-harness-${SUFFIX}.tar.gz"
tar -xzf agent-harness.tar.gz
mv "agent-harness-${SUFFIX}" $PREFIX/bin/agent-harness
chmod +x $PREFIX/bin/agent-harness
```

### Setting Up the `ah` Alias

Add to `~/.bashrc` or `~/.zshrc`:

```bash
# Agent Harness workspace shortcut
ah() {
    if [ $# -eq 0 ]; then
        agent-harness
    else
        agent-harness "$@"
    fi
}
```

Then reload your shell:
```bash
source ~/.bashrc  # or source ~/.zshrc
```

---

## Docker

### Using Pre-built Images (Coming Soon)

```bash
# Pull the image
docker pull ghcr.io/ba-calderonmorales/agent-harness:latest

# Run interactively
docker run -it --rm \
  -v $(pwd):/workspace \
  -e OPENROUTER_API_KEY=$OPENROUTER_API_KEY \
  ghcr.io/ba-calderonmorales/agent-harness:latest
```

### Building Your Own Image

```bash
# Clone the repository
git clone https://github.com/BA-CalderonMorales/agent-harness.git
cd agent-harness

# Build the image
docker build -t agent-harness:latest .

# Run
docker run -it --rm -v $(pwd):/workspace agent-harness:latest
```

### Docker Compose

Create a `docker-compose.yml`:

```yaml
version: '3.8'

services:
  agent-harness:
    image: ghcr.io/ba-calderonmorales/agent-harness:latest
    volumes:
      - .:/workspace
      - ~/.agent-harness:/home/agent/.agent-harness
    environment:
      - OPENROUTER_API_KEY=${OPENROUTER_API_KEY}
    working_dir: /workspace
    stdin_open: true
    tty: true
```

Run with:
```bash
docker-compose run --rm agent-harness
```

---

## Build from Source

### Prerequisites

- Go 1.26.1 or later
- Git

### Build

```bash
# Clone the repository
git clone https://github.com/BA-CalderonMorales/agent-harness.git
cd agent-harness

# Build
go build -o agent-harness ./cmd/agent-harness

# Or install to $GOPATH/bin
go install ./cmd/agent-harness
```

### Cross-Compilation

```bash
# Build for Linux AMD64
GOOS=linux GOARCH=amd64 go build -o agent-harness-linux-amd64 ./cmd/agent-harness

# Build for macOS ARM64
GOOS=darwin GOARCH=arm64 go build -o agent-harness-darwin-arm64 ./cmd/agent-harness

# Build for Windows AMD64
GOOS=windows GOARCH=amd64 go build -o agent-harness-windows-amd64.exe ./cmd/agent-harness
```

---

## Post-Installation

### Verify Installation

```bash
agent-harness --version
```

### First Run Setup

On first run, you'll be prompted to:

1. **Choose API Provider**: OpenRouter (recommended), OpenAI, or Anthropic
2. **Enter API Key**: Your API key (input will be masked)
3. **Select Model**: Choose from available models for your provider
4. **Set Master Password**: Create a password to encrypt your credentials

Your credentials are encrypted with AES-256-GCM and stored at `~/.agent-harness/credentials.enc` with 0600 permissions.

### Configuration Directory

agent-harness uses the following directories:

- **Config**: `~/.agent-harness/settings.json`
- **Sessions**: `~/.agent-harness/sessions/`
- **Skills**: `./.agent-harness/skills/`
- **Local Config**: `./.agent-harness/settings.local.json`

---

## Troubleshooting

### Permission Denied

If you get a "permission denied" error:

```bash
# Make sure the binary is executable
chmod +x /path/to/agent-harness

# Or use sudo for system-wide installation
sudo mv agent-harness /usr/local/bin/
```

### Command Not Found

If `agent-harness` is not in your PATH:

```bash
# Check if it's installed
which agent-harness

# If not found, add to PATH
export PATH="$HOME/.local/bin:$PATH"

# Or for system-wide
export PATH="/usr/local/bin:$PATH"
```

### Credential Encryption Issues

If you forget your master password:

```bash
# Remove the encrypted credentials
rm ~/.agent-harness/credentials.enc

# Run setup again
agent-harness
```

### API Key Not Working

1. Verify your API key is correct
2. Check that you have credits/quota with your provider
3. Try setting the environment variable directly:
   ```bash
   export OPENROUTER_API_KEY="sk-or-v1-..."
   agent-harness
   ```

### Getting Help

```bash
# Show help
agent-harness --help

# Show version
agent-harness --version

# Run with verbose output
agent-harness --verbose
```

---

## Uninstallation

### Remove Binary

```bash
# If installed to /usr/local/bin
sudo rm /usr/local/bin/agent-harness

# If installed to ~/.local/bin
rm ~/.local/bin/agent-harness
```

### Remove Configuration

```bash
# Remove all configuration and sessions
rm -rf ~/.agent-harness
```

### Remove Project Settings

```bash
# Remove project-specific settings
rm -rf ./.agent-harness
```
