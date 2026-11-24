---
name: self-audit
description: Audit one file or package of this repository against a fixed quality model, report findings with line numbers, and propose a patch in a scratch copy that is validated by the repository's own tests. Use when asked to review a module for bugs, size budget violations, unused code, missing tests or inefficient patterns, or to prepare a candidate self-improvement. Never commits, pushes or edits the working tree.
---

# Self-Audit

> **Purpose:** Let the harness review its own code the way a reviewer would, with explicit rules and existing tests as the judge. The output is a report and a patch. A human decides what lands.

## Hard Rules

1. **Read-only on the working tree.** Do not edit, create or delete files outside the scratch directory described below.
2. **No commits, pushes, tags, PRs or settings changes.** Those are the user's decisions.
3. **One target per run.** A target is one file or one package directory. Refuse a whole-repository scope and ask for a narrower one.
4. **Human checkpoint for architecture.** If a proposed change touches a public interface, the settings or permission model, persistence formats, or the agent loop's control flow, stop after the report and ask for sign-off. Do not write a patch for it.
5. **Validation is the judge.** A proposed patch is only "better" if the repository's own checks pass on the scratch copy and no check that passed before now fails.
6. **Stay inside budget.** Read at most the target plus its direct test file and the one file that defines the rule being checked. Report at most 10 findings per run; rank them.

## Inputs

- `target`: a file or package path, for example `internal/core/record/render.go` or `internal/core/record`.
- `checks` (optional): a subset of the quality model below. Default: all.

## Quality Model

Each rule is checkable from code and repository tooling. A finding must cite the rule id.

| Id | Rule | How to check |
|---|---|---|
| Q1 | Files stay at or under 400 lines (test and spec files are exempt) | `wc -l` on the target; `make verify-structure` |
| Q2 | Every exported function that has branches has a test that fails without it | grep for `_test.go` names that mention the function; mutation check on one branch |
| Q3 | No unused parameters, fields or functions | `go vet ./...`; `staticcheck` on the package |
| Q4 | Errors are returned or handled, not discarded | grep for `_ = ` and `, _ :=` on calls that return errors |
| Q5 | Hard-coded paths, host names or model ids live in a defaults file | grep the target for literal URLs, `127.0.0.1`, and model ids; compare with `internal/core/config/defaults.go` |
| Q6 | Each file has one concept; a new file name matches its concept | read the package doc and the top-level declarations |
| Q7 | Terminal rendering stays inside the narrow width the tests use (40 columns) | grep for fixed widths; check against the ttui tests |
| Q8 | No dead code paths behind constant conditions | read conditionals whose operands are constants |

## Procedure

**1. Scope.** Resolve `target` to a list of Go files. Refuse if it matches more than 12 non-test files. Record the file sizes and the names of the direct test files.

**2. Scan.** Apply the checks from the quality model. For each finding, record: rule id, file, line, the exact line text, a one-sentence reason, and a severity (`error` for correctness or a failing gate, `warning` for a budget or style rule, `note` for a judgement call).

**3. Baseline.** Before any change, run the target's package tests in a scratch copy and record which pass:

```bash
mkdir -p tmp/self-audit
git worktree add --detach tmp/self-audit/base HEAD
(cd tmp/self-audit/base && GOTMPDIR="$PWD/../gotmp" go test ./<package-of-target>/... -count=1)
```

Record the pass and fail list. If the baseline already fails, stop and report that; do not propose changes on a red baseline.

**4. Propose.** For each `error` or `warning` finding that is not an architecture item (rule 4), write the smallest change that fixes it. Apply it only in a second scratch worktree:

```bash
git worktree add --detach tmp/self-audit/candidate HEAD
```

One finding per change. Keep the change inside the target. Do not change tests to make them pass.

**5. Validate.** In the candidate worktree run, in order:

- `gofmt -l <target>` (must be empty)
- `go vet ./<package-of-target>/...`
- `go test ./<package-of-target>/... -count=1` (the same package list as the baseline)
- `make verify-structure` (no new file over 400 lines)

A candidate is accepted only if every command passes and no baseline test that passed now fails. For a rule-Q2 finding, also show that the new or existing test goes red when the fixed branch is reverted.

**6. Report.** Write one JSON document to `tmp/self-audit/report.json`:

```json
{
  "target": "internal/core/record/render.go",
  "baseline": {"packages": ["./internal/core/record"], "passed": true},
  "findings": [
    {
      "rule": "Q4",
      "file": "internal/core/record/render.go",
      "line": 42,
      "severity": "warning",
      "reason": "error from strconv.Atoi discarded",
      "status": "accepted | rejected | needs_signoff",
      "validation": {"gofmt": "pass", "vet": "pass", "tests": "pass", "structure": "pass"},
      "patch": "tmp/self-audit/patches/Q4-render-go-42.patch"
    }
  ]
}
```

Write each accepted change as a patch file under `tmp/self-audit/patches/`. Do not apply patches to the working tree.

**7. Clean up.** Keep `tmp/self-audit/` for the user to review. Remove only the worktrees you created (`git worktree remove`).

## Output to the User

Give a short summary: the target, the baseline result, a table of findings with status, the list of patch files, and any `needs_signoff` items with the reason. Say plainly that nothing was committed.

## What This Skill Does Not Do

- It does not generate or run an unbounded self-improvement loop. Each run is one target, reviewed by a person.
- It does not measure performance. Latency and memory need a separate benchmark; do not claim "faster" from this skill.
- It does not replace `evals/`. A change to terminal behaviour needs the ttui tests and, for model-facing changes, the eval suite.
