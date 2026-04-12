#!/bin/bash
# setup-branch-protection.sh - Apply branch protection to all workspace repos
# Usage: ./scripts/setup-branch-protection.sh [--dry-run]

set -e

DRY_RUN=false
if [ "$1" = "--dry-run" ]; then
    DRY_RUN=true
    echo "◆ DRY RUN MODE"
fi

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Projects directory
PROJECTS_DIR="$HOME/projects"

# Special cases with custom protected branches
declare -A SPECIAL_PROTECTED
declare -A SPECIAL_DEFAULT
declare -A SPECIAL_DEVELOP

# my-life-as-a-dev: main + gh-pages (no develop workflow)
SPECIAL_PROTECTED["my-life-as-a-dev"]="main gh-pages"
SPECIAL_DEFAULT["my-life-as-a-dev"]="main"
SPECIAL_DEVELOP["my-life-as-a-dev"]=""

# Default for most repos: main + develop
DEFAULT_PROTECTED="main develop"
DEFAULT_DEFAULT="main"
DEFAULT_DEVELOP="develop"

# Repos to skip (archived, reference, etc.)
SKIP_REPOS=(
    ".workspace"
    "agent-harness-reference"
)

is_skipped() {
    local repo="$1"
    for skip in "${SKIP_REPOS[@]}"; do
        if [ "$repo" = "$skip" ]; then
            return 0
        fi
    done
    return 1
}

get_protected_branches() {
    local repo="$1"
    if [ -n "${SPECIAL_PROTECTED[$repo]}" ]; then
        echo "${SPECIAL_PROTECTED[$repo]}"
    else
        echo "$DEFAULT_PROTECTED"
    fi
}

get_default_branch() {
    local repo="$1"
    if [ -n "${SPECIAL_DEFAULT[$repo]}" ]; then
        echo "${SPECIAL_DEFAULT[$repo]}"
    else
        echo "$DEFAULT_DEFAULT"
    fi
}

get_develop_branch() {
    local repo="$1"
    if [ -n "${SPECIAL_DEVELOP[$repo]}" ]; then
        echo "${SPECIAL_DEVELOP[$repo]}"
    else
        echo "$DEFAULT_DEVELOP"
    fi
}

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

setup_local_protection() {
    local repo="$1"
    local protected_branches="$2"
    
    if [ "$DRY_RUN" = true ]; then
        print_status info "Would configure local git aliases"
        return
    fi
    
    # Enable auto-prune on fetch
    git config --local fetch.prune true
    git config --local fetch.pruneTags false
    
    # Create protection-checked aliases
    local protected_pattern=$(echo "$protected_branches" | tr ' ' '|')
    
    git config --local alias.del "!sh -c 'for b in \"\$@\"; do case \"\$b\" in $protected_pattern) echo \"ERROR: Protected branch \$b cannot be deleted\"; exit 1;; esac; done; git branch -d \"\$@\"' -"
    
    git config --local alias.del-force "!sh -c 'for b in \"\$@\"; do case \"\$b\" in $protected_pattern) echo \"ERROR: Protected branch \$b cannot be deleted\"; exit 1;; esac; done; git branch -D \"\$@\"' -"
    
    # Copy prune script locally if not exists
    if [ ! -f "scripts/prune-branches.sh" ]; then
        mkdir -p scripts
        cp "$PROJECTS_DIR/agent-harness/scripts/prune-branches.sh" scripts/prune-branches.sh 2>/dev/null || true
        chmod +x scripts/prune-branches.sh 2>/dev/null || true
    fi
    
    # Setup prune aliases
    if [ -f "scripts/prune-branches.sh" ]; then
        git config --local alias.prune-merged '!bash scripts/prune-branches.sh'
        git config --local alias.prune-dry '!bash scripts/prune-branches.sh --dry-run'
    fi
    
    # Setup pre-push hook
    if [ ! -f ".git/hooks/pre-push" ]; then
        cat > .git/hooks/pre-push << 'HOOK'
#!/bin/sh
# Prevent deletion of protected branches
protected_branches="PROTECTED_LIST"
remote="$1"
url="$2"

while read local_ref local_sha remote_ref remote_sha
do
    if [ "$local_sha" = "0000000000000000000000000000000000000000" ]; then
        branch_name=$(echo "$remote_ref" | sed 's|refs/heads/||')
        for protected in $protected_branches; do
            if [ "$branch_name" = "$protected" ]; then
                echo "✗ ERROR: Attempting to delete protected branch '$protected'"
                echo "  Bypass: git push --no-verify $remote --delete $protected"
                exit 1
            fi
        done
    fi
done
exit 0
HOOK
        # Replace PROTECTED_LIST with actual branches
        sed -i "s/PROTECTED_LIST/$protected_branches/g" .git/hooks/pre-push
        chmod +x .git/hooks/pre-push
    fi
    
    print_status ok "Local protection configured"
}

setup_github_protection() {
    local repo="$1"
    local protected_branches="$2"
    
    # Get repo full name from remote
    local repo_url=$(git remote get-url origin 2>/dev/null || echo "")
    local repo_full=""
    
    if echo "$repo_url" | grep -q "github.com"; then
        repo_full=$(echo "$repo_url" | sed -E 's/.*github\.com[\/:]//; s/\.git$//')
    fi
    
    if [ -z "$repo_full" ]; then
        print_status warn "Not a GitHub repo, skipping remote protection"
        return
    fi
    
    if [ "$DRY_RUN" = true ]; then
        print_status info "Would configure GitHub protection for $protected_branches"
        return
    fi
    
    # Apply protection to each branch
    for branch in $protected_branches; do
        # Check if branch exists on remote
        if ! git ls-remote --heads origin "$branch" | grep -q "$branch"; then
            print_status warn "Branch '$branch' not on remote, skipping"
            continue
        fi
        
        local result=$(gh api "repos/$repo_full/branches/$branch/protection" \
            --method PUT \
            --input - << EOF 2>&1 || echo "FAILED"
{
  "restrictions": null,
  "enforce_admins": false,
  "required_pull_request_reviews": null,
  "required_status_checks": null,
  "allow_force_pushes": false,
  "allow_deletions": false,
  "required_conversation_resolution": false,
  "lock_branch": false
}
EOF
        )
        
        if echo "$result" | grep -q '"allow_deletions":'; then
            print_status ok "GitHub protection: $branch"
        else
            print_status error "Failed to protect $branch on GitHub"
            echo "$result" | head -3
        fi
    done
}

# Main execution
echo "========================================"
echo "Multi-Repo Branch Protection Setup"
echo "========================================"
echo ""

# Find all git repos
repos=$(find "$PROJECTS_DIR" -maxdepth 2 -name ".git" -type d 2>/dev/null | while read gitdir; do dirname "$gitdir"; done | sort)

total=0
success=0
skipped=0

for repo_path in $repos; do
    repo_name=$(basename "$repo_path")
    total=$((total + 1))
    
    echo ""
    echo "▶ $repo_name"
    
    if is_skipped "$repo_name"; then
        print_status info "Skipping (in skip list)"
        skipped=$((skipped + 1))
        continue
    fi
    
    cd "$repo_path"
    
    # Check if it's actually a git repo with GitHub remote
    if ! git remote get-url origin &>/dev/null; then
        print_status warn "No origin remote, skipping"
        skipped=$((skipped + 1))
        continue
    fi
    
    protected=$(get_protected_branches "$repo_name")
    default=$(get_default_branch "$repo_name")
    develop=$(get_develop_branch "$repo_name")
    
    print_status info "Protected: $protected"
    
    # Setup local protection
    setup_local_protection "$repo_name" "$protected"
    
    # Setup GitHub protection
    if command -v gh &>/dev/null && gh auth status &>/dev/null; then
        setup_github_protection "$repo_name" "$protected"
    else
        print_status warn "GitHub CLI not available, skipping remote protection"
    fi
    
    success=$((success + 1))
done

echo ""
echo "========================================"
echo "Summary"
echo "========================================"
printf "Total repos:  %d\n" "$total"
printf "Configured:   %d\n" "$success"
printf "Skipped:      %d\n" "$skipped"
echo ""

if [ "$DRY_RUN" = true ]; then
    echo "This was a dry run. Run without --dry-run to apply changes."
fi
