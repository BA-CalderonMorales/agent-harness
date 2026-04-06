# Release Patterns Quick Reference

> One-page reference for release workflow patterns.

---

## The Golden Rules

1. **Isolated Commits** - Each step = one commit
2. **CI-First** - Wait for green before next step  
3. **Always Return to Develop** - Never end on main

---

## The 5-Step Release

```bash
# 1. Version bump (develop)
sed -i 's/Version   = "X.X.X"/Version   = "X.X.Y"/' cmd/*/main.go
git add -A && git commit -m "chore(release): bump version to vX.X.Y"
git push origin develop

# 2. Wait for CI, then merge to main
git checkout main && git merge develop && git push origin main

# 3. Wait for CI, then tag
git tag -a "vX.X.Y" -m "Release vX.X.Y"
git push origin "vX.X.Y"

# 4. Wait for release workflow
gh run watch

# 5. CRITICAL: Return to develop
git checkout develop
```

---

## Version Alignment Check

```bash
CODE=$(grep -E 'Version\s*=\s*"[^"]+"' cmd/*/main.go | sed 's/.*"\([^"]*\)".*/\1/')
GIT=$(git describe --tags --abbrev=0 | sed 's/^v//')
GH=$(gh release list --limit 1 --json tagName -q '.[0].tagName' | sed 's/^v//')

echo "Code: $CODE | Git: $GIT | GitHub: $GH"
[ "$CODE" = "$GIT" ] && [ "$GIT" = "$GH" ] && echo "[OK]" || echo "[!] Mismatch"
```

---

## Cross-Repo Sync Checklist

When releasing multiple harnesses:

- [ ] agent-harness released
- [ ] lumina-bot released  
- [ ] terminal-jarvis released
- [ ] All versions aligned
- [ ] All CI green

---

## Emergency Hotfix

```bash
git checkout main
git checkout -b hotfix/critical
# Fix + version bump (same commit)
git commit -m "fix: critical issue + bump vX.X.Y-hotfix1"
git checkout main && git merge hotfix/critical
git tag -a "vX.X.Y-hotfix1" -m "Hotfix vX.X.Y-hotfix1"
git push origin main --follow-tags
git checkout develop && git merge main && git push origin develop
```

---

## Remember

> "Isolated commits, CI-first, always return to develop."
