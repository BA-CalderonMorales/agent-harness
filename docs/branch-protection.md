# Branch Protection Setup

This document describes the branch protection configuration for agent-harness.

## Local Protection (Configured)

### Auto-Prune on Fetch
```bash
git config --local fetch.prune true
```
Automatically prunes remote-tracking branches when fetching.

### Git Aliases (Safe Delete)
```bash
# Safe branch deletion (checks protection)
git del <branch>        # Same as git branch -d with protection
git del-force <branch>  # Same as git branch -D with protection

# Prune merged branches
git prune-merged        # Delete branches merged to main/develop
git prune-dry           # Preview what would be deleted
```

### Pre-Push Hook
Prevents accidental deletion of `main` and `develop` branches on push.
Located at: `.git/hooks/pre-push`

To bypass (emergency only):
```bash
git push --no-verify origin --delete <branch>
```

## GitHub Protection (Manual Setup Required)

You must configure these in GitHub repository settings:

### 1. Navigate to Settings
Repository → Settings → Branches

### 2. Add Protection Rule for `main`

| Setting | Value |
|---------|-------|
| Branch name pattern | `main` |
| Restrict deletions | ✓ Enabled |
| Require pull request reviews | Optional (for this repo) |
| Require status checks to pass | Optional |
| Require conversation resolution | Optional |
| Include administrators | Optional |

### 3. Add Protection Rule for `develop`

| Setting | Value |
|---------|-------|
| Branch name pattern | `develop` |
| Restrict deletions | ✓ Enabled |
| Require pull request reviews | Optional |
| Require status checks to pass | Optional |
| Require conversation resolution | Optional |
| Include administrators | Optional |

## Using the Prune Script

### Dry Run (Recommended First)
```bash
./scripts/prune-branches.sh --dry-run
```

### Actual Prune
```bash
./scripts/prune-branches.sh
```

### What Gets Deleted
- Local branches merged into `main` or `develop`
- Remote branches merged into `origin/main` or `origin/develop`
- Does NOT delete: `main`, `develop`, or unmerged branches

## Protected Branches

| Branch | Protection |
|--------|------------|
| `main` | Cannot delete locally or remotely |
| `develop` | Cannot delete locally or remotely |

## Emergency Override

If you absolutely need to delete a protected branch:

```bash
# Remove local hook temporarily
mv .git/hooks/pre-push .git/hooks/pre-push.bak

# Delete branch
git push origin --delete develop

# Restore hook
mv .git/hooks/pre-push.bak .git/hooks/pre-push
```

**Warning**: Only use this in extreme emergencies.
