#!/bin/bash
# block-user.sh - Block a GitHub user from interacting with your repositories
# Usage: ./scripts/block-user.sh <username>
#
# Requires: gh auth refresh -s user (one-time setup for user blocking scope)

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

USER="${1:-}"

if [ -z "$USER" ]; then
    echo "Usage: $0 <github-username>"
    echo "Example: $0 superman32432432"
    exit 1
fi

echo "========================================"
echo "GitHub User Block"
echo "========================================"
echo "Target: $USER"
echo ""

# Check if already blocked
if gh api "/user/blocks/$USER" >/dev/null 2>&1; then
    echo -e "${GREEN}✓${NC} $USER is already blocked"
    exit 0
fi

# Attempt block
if gh api -X PUT "/user/blocks/$USER" >/dev/null 2>&1; then
    echo -e "${GREEN}✓${NC} Blocked $USER successfully"
    echo ""
    echo "Effect:"
    echo "  - User cannot interact with your repositories"
    echo "  - User cannot send you notifications"
    echo "  - User cannot fork your private repos"
    echo "  - Existing contributions remain attributed (GitHub limitation)"
else
    echo -e "${YELLOW}⚠${NC} Block failed — likely missing 'user' scope"
    echo ""
    echo "Run this to grant the required scope:"
    echo "  gh auth refresh -h github.com -s user"
    echo ""
    echo "Then re-run: $0 $USER"
    exit 1
fi
