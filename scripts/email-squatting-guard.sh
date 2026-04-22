#!/bin/bash
# email-squatting-guard.sh - Detect and prevent email squatting attacks
# Usage: ./scripts/email-squatting-guard.sh [--check-all | --fix-config | --audit]
#
# Checks for:
#   - Placeholder emails in local/global git config
#   - Suspicious contributor attribution on GitHub
#   - Commits using non-noreply emails that could be squatted

set -e

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

GITHUB_USER="BA-CalderonMorales"
SAFE_EMAIL="62074841+BA-CalderonMorales@users.noreply.github.com"
PLACEHOLDERS=(
    "user@example.com"
    "test@test.com"
    "foo@bar.com"
    "email@example.com"
    "admin@admin.com"
)

cmd="${1:---check-all}"

check_git_configs() {
    echo "▶ Checking git configurations"
    local global_email
    global_email=$(git config --global user.email || true)

    if [ "$global_email" = "$SAFE_EMAIL" ]; then
        print_status ok "Global git email is safe"
    else
        print_status error "Global git email is unsafe: $global_email"
        print_status info "Fix: git config --global user.email \"$SAFE_EMAIL\""
    fi

    local found_bad=0
    for d in ~/projects/*/.git; do
        [ -d "$d" ] || continue
        local repo
        repo=$(dirname "$d")
        local name
        name=$(basename "$repo")
        local email
        email=$(cd "$repo" && git config user.email || true)

        for ph in "${PLACEHOLDERS[@]}"; do
            if [ "$email" = "$ph" ]; then
                print_status error "$name uses placeholder: $email"
                found_bad=$((found_bad + 1))
            fi
        done
    done

    if [ "$found_bad" -eq 0 ]; then
        print_status ok "All local repo emails are safe"
    else
        print_status warn "$found_bad repo(s) need fixing"
    fi
}

audit_commits() {
    echo ""
    echo "▶ Auditing commit history for unsafe emails"
    local found_bad=0
    for d in ~/projects/*/.git; do
        [ -d "$d" ] || continue
        local repo
        repo=$(dirname "$d")
        local name
        name=$(basename "$repo")
        local bad_emails
        bad_emails=$(cd "$repo" && git log --all --format="%ae" | sort | uniq -c | sort -rn | grep -E "(example\.com|test\.com|localhost)" || true)

        if [ -n "$bad_emails" ]; then
            print_status warn "$name has commits with unsafe emails:"
            echo "$bad_emails" | sed 's/^/    /'
            found_bad=$((found_bad + 1))
        fi
    done

    if [ "$found_bad" -eq 0 ]; then
        print_status ok "No unsafe emails found in commit history"
    fi
}

check_github_contributors() {
    echo ""
    echo "▶ Checking GitHub contributor graphs"
    local repos=("agent-harness" "terminal-jarvis")
    for repo in "${repos[@]}"; do
        local contributors
        contributors=$(gh api "repos/$GITHUB_USER/$repo/contributors" --jq '.[] | "\(.login): \(.contributions)"' 2>/dev/null || true)

        if [ -z "$contributors" ]; then
            print_status warn "$repo: no contributor data"
            continue
        fi

        local suspicious
        suspicious=$(echo "$contributors" | grep -iv "BA-CalderonMorales\|dependabot\|github-actions" || true)

        if [ -n "$suspicious" ]; then
            print_status error "$repo: suspicious contributors detected"
            echo "$suspicious" | sed 's/^/    /'
        else
            print_status ok "$repo: no suspicious contributors"
        fi
    done
}

fix_all_configs() {
    echo "▶ Fixing all git configurations"
    git config --global user.email "$SAFE_EMAIL"
    print_status ok "Global config fixed"

    for d in ~/projects/*/.git; do
        [ -d "$d" ] || continue
        local repo
        repo=$(dirname "$d")
        local name
        name=$(basename "$repo")
        cd "$repo"
        git config user.email "$SAFE_EMAIL"
        print_status ok "$name fixed"
    done
}

case "$cmd" in
    --check-all)
        echo "========================================"
        echo "Email Squatting Guard"
        echo "========================================"
        echo ""
        check_git_configs
        audit_commits
        check_github_contributors
        echo ""
        echo "========================================"
        echo "Audit Complete"
        echo "========================================"
        ;;
    --fix-config)
        fix_all_configs
        ;;
    --audit)
        audit_commits
        ;;
    *)
        echo "Usage: $0 [--check-all | --fix-config | --audit]"
        exit 1
        ;;
esac
