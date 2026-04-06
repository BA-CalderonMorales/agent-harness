---
name: release-patterns
description: Release workflow patterns for agent-harness and related projects. Ensures consistent versioning, tagging, and GitHub release creation across all harness repositories.
---

# Release Patterns

> **Purpose:** Consistent, error-free releases across all agent harness projects.

---

## Pattern 1: Version Alignment Check

Before any release, verify all version sources match:

```bash
#!/bin/bash
# Check version alignment

REPO_DIR="${1:-~/projects/agent-harness}"
cd "$REPO_DIR" || exit 1

# Get code version (Go projects)
CODE_VERSION=$(grep -E 'Version\s*=\s*"[^"]+"' cmd/*/main.go 2>/dev/null | head -1 | sed 's/.*"\([^"]*\)".*/\1/')

# Get latest git tag
GIT_TAG=$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//')

# Get latest GitHub release
GH_RELEASE=$(gh release list --limit 1 --json tagName -q '.[0].tagName' 2>/dev/null | sed 's/^v//')

echo "Code:   $CODE_VERSION"
echo "Git:    $GIT_TAG"
echo "GitHub: $GH_RELEASE"

if [ "$CODE_VERSION" = "$GIT_TAG" ] && [ "$GIT_TAG" = "$GH_RELEASE" ]; then
    echo "[OK] All versions aligned"
    exit 0
else
    echo "[!] Version mismatch detected"
    exit 1
fi
```

---

## Pattern 2: Isolated Release Commits

Each release step must be an isolated commit:

```bash
# Step 1: Version bump (isolated commit)
sed -i 's/Version   = "0.0.51"/Version   = "0.0.52"/' cmd/*/main.go
git add cmd/*/main.go
git commit -m "chore(release): bump version to v0.0.52"

# Step 2: Push to develop
git push origin develop

# Step 3: Merge to main (separate commit)
git checkout main
git merge develop
git push origin main

# Step 4: Tag (annotated tag)
git tag -a "v0.0.52" -m "Release v0.0.52"
git push origin "v0.0.52"

# Step 5: Return to develop
git checkout develop
```

**Never combine these steps into one commit.**

---

## Pattern 3: CI-First Release Flow

```
1. Push version bump to develop
   ↓ (wait for CI)
2. CI passes → Merge to main
   ↓ (wait for CI)
3. CI passes → Create tag
   ↓ (triggers release workflow)
4. Release workflow builds artifacts
   ↓
5. GitHub release created automatically
```

---

## Pattern 4: Post-Release Verification

After tag push, verify within 5 minutes:

```bash
# Watch release workflow
gh run watch

# Verify release created
gh release view v0.0.52

# List artifacts
gh release view v0.0.52 --json assets -q '.assets[].name'
```

---

## Pattern 5: Hotfix Release (Emergency)

When main is broken and needs immediate fix:

```bash
# 1. Create hotfix branch from main
git checkout main
git checkout -b hotfix/critical-fix

# 2. Apply minimal fix
git add -A
git commit -m "fix: critical issue description"

# 3. Version bump (same commit as fix for hotfix)
git add -A
git commit -m "chore(release): bump version to v0.0.52-hotfix1"

# 4. Merge to main and tag immediately
git checkout main
git merge hotfix/critical-fix
git tag -a "v0.0.52-hotfix1" -m "Hotfix release v0.0.52-hotfix1"
git push origin main --follow-tags

# 5. Merge back to develop
git checkout develop
git merge main
git push origin develop
```

---

## Pattern 6: Multi-Repo Sync

When releasing multiple harness projects:

```bash
# Release order (dependencies first)
PROJECTS="agent-harness agent-harness-reference"

for project in $PROJECTS; do
    echo "=== Releasing $project ==="
    cd ~/projects/$project
    
    # Run isolated release for each
    ./scripts/release.sh patch
    
    # Wait for CI before next
    gh run watch --exit-status
done
```

---

## Anti-Patterns

### ❌ Combined Version+Feature Commit
```bash
# Don't do this:
git commit -m "feat: new feature + version bump"
```

### ❌ Tag Before CI Passes
```bash
# Don't do this:
git push origin develop
git tag v0.0.52  # ← CI hasn't passed yet!
```

### ❌ Light Tags
```bash
# Don't do this:
git tag v0.0.52  # Light tag (no message)

# Do this:
git tag -a v0.0.52 -m "Release v0.0.52"
```

---

## Release Checklist Template

```markdown
## Release vX.X.X

- [ ] Version bump committed to develop
- [ ] CI passes on develop
- [ ] Merged to main
- [ ] CI passes on main
- [ ] Annotated tag created and pushed
- [ ] Release workflow completes successfully
- [ ] GitHub release created with artifacts
- [ ] Returned to develop branch
- [ ] VERSION file updated (if applicable)
```

---

> **Remember:** Isolated commits, CI-first, always return to develop.
