# GitNexus tooling

This local installation pins GitNexus 1.6.11 and its dependency lockfile.
Run from the repository root:

```sh
npm ci --prefix scripts/gitnexus
node scripts/gitnexus/node_modules/gitnexus/dist/cli/index.js analyze --index-only --pdg
node scripts/gitnexus/node_modules/gitnexus/dist/cli/index.js query 'tool dispatch' --repo .
npm audit --prefix scripts/gitnexus
```

Use the explicit local CLI path for analysis, impact checks, and change
detection. The generated `.gitnexus/run.cjs` may choose a global installation
or a package-manager cache, which does not use this project's overrides.

The overrides patch dependencies pulled in by optional local embeddings:

- `adm-zip` 0.6.0 fixes [GHSA-xcpc-8h2w-3j85](https://github.com/advisories/GHSA-xcpc-8h2w-3j85).
- `sharp` 0.35.4 includes the fixes for [GHSA-f88m-g3jw-g9cj](https://github.com/advisories/GHSA-f88m-g3jw-g9cj).

To update, check `npm view gitnexus version`, install that stable release
with `npm install --prefix scripts/gitnexus --save-exact gitnexus@VERSION`,
then audit and rebuild the index. Keep overrides until upstream dependency
ranges resolve patched versions without them. Verify CLI indexing and query
behavior after each update; a clean audit alone does not prove compatibility.
