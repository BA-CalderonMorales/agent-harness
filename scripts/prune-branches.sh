#!/bin/bash
# prune-branches.sh - Safely remove branches merged into main or develop
# Usage: ./scripts/prune-branches.sh [--dry-run]

set -e

DRY_RUN=false
if [ "$1" = "--dry-run" ]; then
    DRY_RUN=true
    echo "◆ DRY RUN MODE - No branches will actually be deleted"
    echo ""
fi

# Protected branches - never delete these
PROTECTED_BRANCHES="main develop"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Enable colors
printf '%b' '' > /dev/null 2>&1 || true

echo "→ Fetching latest from remote..."
git fetch --prune origin 2>/dev/null || true

echo ""
echo "→ Checking for merged branches..."
echo ""

# Function to check if branch is protected
is_protected() {
    local branch="$1"
    for protected in $PROTECTED_BRANCHES; do
        if [ "$branch" = "$protected" ]; then
            return 0
        fi
    done
    return 1
}

# Track if we found any branches to delete
FOUND_BRANCHES=false

print_status() {
    local color="$1"
    local msg="$2"
    printf "    %b%s%b %s\n" "$color" "$msg" "$NC"
}

# Check local branches merged into main
echo "  Local branches merged into main:"
for branch in $(git branch --merged main --format='%(refname:short)' 2>/dev/null || true); do
    if ! is_protected "$branch"; then
        FOUND_BRANCHES=true
        if [ "$DRY_RUN" = true ]; then
            print_status "$YELLOW" "[would delete]" "$branch"
        else
            print_status "$GREEN" "[deleting]" "$branch"
            git branch -d "$branch" 2>/dev/null || print_status "$RED" "[failed]" "$branch"
        fi
    fi
done

# Check local branches merged into develop
echo ""
echo "  Local branches merged into develop:"
for branch in $(git branch --merged develop --format='%(refname:short)' 2>/dev/null || true); do
    if ! is_protected "$branch"; then
        FOUND_BRANCHES=true
        if [ "$DRY_RUN" = true ]; then
            print_status "$YELLOW" "[would delete]" "$branch"
        else
            print_status "$GREEN" "[deleting]" "$branch"
            git branch -d "$branch" 2>/dev/null || print_status "$RED" "[failed]" "$branch"
        fi
    fi
done

# Check remote branches (that don't exist locally) merged into origin/main
echo ""
echo "  Remote branches merged into origin/main:"
for branch in $(git branch -r --merged origin/main --format='%(refname:short)' 2>/dev/null | grep -v 'HEAD' | sed 's|origin/||' || true); do
    if ! is_protected "$branch"; then
        # Skip if local branch exists (already handled above)
        if git show-ref --verify --quiet "refs/heads/$branch" 2>/dev/null; then
            continue
        fi
        FOUND_BRANCHES=true
        if [ "$DRY_RUN" = true ]; then
            print_status "$YELLOW" "[would delete remote]" "origin/$branch"
        else
            print_status "$GREEN" "[deleting remote]" "origin/$branch"
            git push origin --delete "$branch" 2>/dev/null || print_status "$RED" "[failed]" "origin/$branch"
        fi
    fi
done

# Check remote branches merged into origin/develop
echo ""
echo "  Remote branches merged into origin/develop:"
for branch in $(git branch -r --merged origin/develop --format='%(refname:short)' 2>/dev/null | grep -v 'HEAD' | sed 's|origin/||' || true); do
    if ! is_protected "$branch"; then
        # Skip if local branch exists
        if git show-ref --verify --quiet "refs/heads/$branch" 2>/dev/null; then
            continue
        fi
        FOUND_BRANCHES=true
        if [ "$DRY_RUN" = true ]; then
            print_status "$YELLOW" "[would delete remote]" "origin/$branch"
        else
            print_status "$GREEN" "[deleting remote]" "origin/$branch"
            git push origin --delete "$branch" 2>/dev/null || print_status "$RED" "[failed]" "origin/$branch"
        fi
    fi
done

echo ""
if [ "$FOUND_BRANCHES" = false ]; then
    echo "✓ No merged branches found to prune."
else
    if [ "$DRY_RUN" = true ]; then
        echo ""
        echo "Run without --dry-run to actually delete these branches."
    else
        echo "✓ Pruning complete."
    fi
fi

echo ""
echo "→ Protected branches (never deleted): $PROTECTED_BRANCHES"
