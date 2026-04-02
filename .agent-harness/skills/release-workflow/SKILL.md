# Release Workflow Skill - Agent Harness

> **Purpose:** Ensure consistent, reliable releases every time. No missed workflows, no manual errors, no surprises.

---

## 1. The Golden Rule

**Always use the Makefile. Never bypass it.**

The Makefile contains critical build flags (`ldflags`) that inject version info. Building manually with `go build` creates broken binaries that report wrong versions.

---

## 2. Quick Reference

```bash
# Step 1: Ensure you're on develop with clean changes
git checkout develop
git status  # Should be clean

# Step 2: Run tests
make test

# Step 3: Bump version in cmd/agent-harness/main.go
# Edit: var Version = "X.X.X"

# Step 4: Commit the version bump
git add -A
git commit -m "chore(version): bump to vX.X.X"

# Step 5: Build locally to verify
make build
./build/agent-harness --version  # Should show vX.X.X

# Step 6: Push to develop
git push origin develop

# Step 7: Tag and push (this triggers the release workflow)
git tag -a vX.X.X -m "Release vX.X.X - Brief description"
git push origin vX.X.X  # <-- CRITICAL: This triggers GitHub Actions

# Step 8: Monitor the release
gh run watch  # or check GitHub Actions UI
```

---

## 3. Common Failures & Prevention

### Failure: Workflow Didn't Trigger

**Symptom:** Tag exists locally, no GitHub Actions run.

**Cause:** Forgot to `git push origin vX.X.X`

**Prevention:** 
```bash
# Always verify tag was pushed
git ls-remote --tags origin | grep "vX.X.X"

# Or use this helper function (add to .bashrc)
release-tag() {
    local version=$1
    local message=${2:-"Release $version"}
    
    if [ -z "$version" ]; then
        echo "Usage: release-tag vX.X.X 'Release description'"
        return 1
    fi
    
    echo "Creating tag $version..."
    git tag -a "$version" -m "$message"
    
    echo "Pushing to origin..."
    git push origin "$version"
    
    echo "Verifying..."
    sleep 2
    git ls-remote --tags origin | grep "$version" || echo "WARNING: Tag not found on remote!"
}
```

### Failure: Wrong Version in Binary

**Symptom:** Binary shows old version or "dev" despite new tag.

**Cause:** Used `go build` instead of `make build`.

**Prevention:** Always use the Makefile:
```bash
make build  # Correct
./build/agent-harness --version
```

---

## 4. Release Checklist

Use this before every release:

```markdown
- [ ] On `develop` branch with clean working directory
- [ ] All tests pass: `make test`
- [ ] Local build works: `make build && ./build/agent-harness --version`
- [ ] Version string updated in `cmd/agent-harness/main.go`
- [ ] Version bump committed: `git commit -m "chore(version): bump to vX.X.X"`
- [ ] Changes pushed to origin: `git push origin develop`
- [ ] Tag created: `git tag -a vX.X.X -m "Release vX.X.X"`
- [ ] Tag pushed to origin: `git push origin vX.X.X`
- [ ] Verify tag on remote: `git ls-remote --tags origin | grep vX.X.X`
- [ ] GitHub Actions workflow triggered (check Actions tab)
- [ ] All matrix builds succeed
- [ ] Release created with artifacts
```

---

## 5. Understanding the Workflow

### Trigger Conditions

The release workflow (`.github/workflows/release.yml`) triggers on:

```yaml
on:
  push:
    tags:
      - 'v*'  # Any tag starting with 'v'
  workflow_dispatch:  # Manual trigger fallback
```

**Key insight:** The workflow triggers on `push`, not `tag create`. Local tags don't count.

### Build Matrix

The workflow builds for:
- Linux (AMD64, ARM64)
- macOS (AMD64 Intel, ARM64 Apple Silicon)
- Windows (AMD64, ARM64)

---

## 6. Emergency Procedures

### Workflow Failed Mid-Release

```bash
# Check what failed
gh run list --workflow=release.yml

# If it's a transient issue, re-run:
gh run rerun <run-id>

# If the tag is bad, delete and recreate:
git push --delete origin vX.X.X  # Delete remote tag
git tag -d vX.X.X                 # Delete local tag

# Fix issues, then re-tag
# ... make fixes ...
git tag -a vX.X.X -m "Release vX.X.X - fixed"
git push origin vX.X.X
```

### Wrong Commit Tagged

```bash
# Delete bad tag everywhere
git push --delete origin vX.X.X
git tag -d vX.X.X

# Recreate at correct commit
git tag -a vX.X.X <commit-sha> -m "Release vX.X.X"
git push origin vX.X.X
```

---

## 7. Version Numbering

Follow semantic versioning:

| Pattern | Meaning | Example |
|---------|---------|---------|
| `v0.X.0` | Minor release, new features | `v0.17.0` |
| `v0.0.X` | Patch release, bug fixes | `v0.17.1` |
| `v0.X.0-alpha` | Pre-release | `v0.18.0-alpha` |

**For this project (pre-1.0):**
- Bump minor for UX changes, new features
- Bump patch for bug fixes
