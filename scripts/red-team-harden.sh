#!/bin/bash
# red-team-harden.sh - Aggressive branch protection for deployment-sensitive repos
# Usage: ./scripts/red-team-harden.sh [--dry-run]
#
# Applies red-team containment rules:
#   - Require PR + 1 approving review (dismiss stale)
#   - Require status checks to pass (strict / up-to-date)
#   - Block force pushes
#   - Block branch deletion
#   - Enforce for admins
#   - Require conversation resolution
#
# Targets: agent-harness, terminal-jarvis, terminal-jarvis-landing,
#          terminal-screensaver, homebrew-terminal-jarvis, MiroFish

set -e

DRY_RUN=false
if [ "$1" = "--dry-run" ]; then
    DRY_RUN=true
    echo "◆ DRY RUN MODE"
fi

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_status() {
    local status="$1"
    local msg="$2"
    case "$status" in
        ok) printf "  %b✓%b %s\n" "$GREEN" "$NC" "$msg" ;;
        warn) printf "  %b⚠%b %s\n" "$YELLOW" "$NC" "$msg" ;;
        error) printf "  %b✗%b %s\n" "$RED" "$NC" "$msg" ;;
        info) printf "  %b→%b %s\n" "$BLUE" "$NC" "$msg" ;;
    esac
}

REPOS=(
    "BA-CalderonMorales/agent-harness"
    "BA-CalderonMorales/terminal-jarvis"
    "BA-CalderonMorales/terminal-jarvis-landing"
    "BA-CalderonMorales/terminal-screensaver"
    "BA-CalderonMorales/homebrew-terminal-jarvis"
    "BA-CalderonMorales/MiroFish"
)

# Branch protection payload (red-team mode)
protect_branch() {
    local repo="$1"
    local branch="$2"
    local checks_json="$3"

    if [ "$DRY_RUN" = true ]; then
        print_status info "Would harden $repo:$branch"
        return
    fi

    local payload
    if [ -n "$checks_json" ]; then
        payload=$(cat <<EOF
{
  "restrictions": null,
  "enforce_admins": true,
  "required_pull_request_reviews": {
    "required_approving_review_count": 1,
    "dismiss_stale_reviews": true,
    "require_code_owner_reviews": false
  },
  "required_status_checks": {
    "strict": true,
    "checks": $checks_json
  },
  "allow_force_pushes": false,
  "allow_deletions": false,
  "required_conversation_resolution": true
}
EOF
)
    else
        payload=$(cat <<EOF
{
  "restrictions": null,
  "enforce_admins": true,
  "required_pull_request_reviews": {
    "required_approving_review_count": 1,
    "dismiss_stale_reviews": true,
    "require_code_owner_reviews": false
  },
  "required_status_checks": null,
  "allow_force_pushes": false,
  "allow_deletions": false,
  "required_conversation_resolution": true
}
EOF
)
    fi

    local result
    result=$(echo "$payload" | gh api "repos/$repo/branches/$branch/protection" --method PUT --input - 2>&1) || true

    if echo "$result" | grep -q '"allow_deletions":'; then
        print_status ok "$repo:$branch hardened"
    else
        print_status error "$repo:$branch failed"
        echo "$result" | head -3
    fi
}

# Per-repo status check configs
checks_agent_harness='[
  {"context": "build", "app_id": 15368},
  {"context": "test", "app_id": 15368},
  {"context": "quality", "app_id": 15368}
]'

checks_terminal_jarvis='[
  {"context": "test", "app_id": 15368},
  {"context": "security-rust", "app_id": 15368},
  {"context": "security-general", "app_id": 15368}
]'

echo "========================================"
echo "Red Team Hardening"
echo "========================================"
echo ""

for repo in "${REPOS[@]}"; do
    echo "▶ $repo"

    # Verify repo exists
    if ! gh api "repos/$repo" >/dev/null 2>&1; then
        print_status warn "Repo not found or no access"
        continue
    fi

    # Determine branches
    repo_branches=$(gh api "repos/$repo/branches" --jq '.[].name' 2>/dev/null || true)
    has_main=false
    has_develop=false
    for b in $repo_branches; do
        [ "$b" = "main" ] && has_main=true
        [ "$b" = "develop" ] && has_develop=true
    done

    # Determine checks
    checks=""
    case "$repo" in
        *agent-harness)
            checks="$checks_agent_harness"
            ;;
        *terminal-jarvis)
            checks="$checks_terminal_jarvis"
            ;;
    esac

    if [ "$has_main" = true ]; then
        protect_branch "$repo" "main" "$checks"
    fi

    if [ "$has_develop" = true ]; then
        protect_branch "$repo" "develop" "$checks"
    fi
done

echo ""
echo "========================================"
echo "Hardening Complete"
echo "========================================"
