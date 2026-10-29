# Everyday capability checks

This is a small baseline for Agent-Harness with `qwen3.5-4b-FLM`, not a
benchmark framework. Start with five situations we have actually encountered.
Do not add a case unless a real failure or product decision needs it.

The goal is an everyday desktop assistant whose routine turns cost almost
nothing. Lemonade with `qwen3.5-4b-FLM` is the current local candidate; hosted
providers stay supported for work the local model cannot do. Keep the checks
portable: judge outcomes, not the wording one model prefers. Do not tune shared
prompts, tool behavior, or acceptance criteria to Qwen, this laptop, or these
five fixtures. When a case goes wrong, fix the harness if the harness is at
fault, and record model limits separately from missing harness capabilities. One
unfamiliar task is worth more than another familiar one. Hosted spending is
always an explicit user choice: the helper never falls back to a paid provider.

| Case | What it establishes | Completion evidence |
| --- | --- | --- |
| `greeting` | Ordinary conversation feels responsive | Short reply, no tools; human timing/UX review |
| `coding` | Read, change, test, report | Four independent tests pass, only `names.py` changes, actual tool/test evidence agrees with the final report |
| `recovery` | A wrong filename does not derail work | Find the real tests, finish the same task, no runaway repetition or test changes |
| `weather` | Current-information requests are handled honestly | A current, relevant source and useful forecast, or an explicitly recorded capability gap; no fabricated answer or coding-topic redirect |
| `cancel` | The user can stop and continue | Esc stops the turn, then one follow-up receives one correct reply without stale updates |

The exact prompts and review criteria live in [cases.json](cases.json).
Weather task completion and honest limitation reporting are separate outcomes:
a truthful refusal is useful, but it does not establish weather capability.

## Commands

Run from the repository root with Lemonade already running and the model
available:

```bash
python evals/run.py                    # usage and examples
python evals/run.py list               # cases with resolved prompts
python evals/run.py run coding         # one case
python evals/run.py run coding recovery
python evals/run.py run --all          # every case, in order
python evals/run.py review PATH         # review a saved case report
python evals/run.py coding             # same as: run coding
```

`run` accepts any number of case names and rejects unknown names before it
builds, prepares a fixture, or contacts the server. `list` fills date
placeholders such as `{today}`, so what you read is the prompt you paste.

A batch builds the app once. Each selected case then runs in order with its own
fixture, Git root, settings, home, data and session directory, plus its own
`report.json`. An ordinary failure does not stop later cases. Ctrl+C saves the
running case as `interrupted`, marks the rest `not_run`, and exits without
starting more work. The batch writes `summary.json` next to the case directories.

`--all` queues cases for you. It does not automate the TUI, paste prompts, grant
approvals, or judge answers. Every case is still a manual session: paste the
prompt, approve only the expected fixture work, and quit the app when the task
is done.

`--prepare-only` writes fixtures and prompts without inference or launching the
app. A failed Lemonade preflight records `blocked`, not a model failure. A clean
machine verification records `needs_review`, not a model pass.

## Reading a result

| Status | Meaning |
| --- | --- |
| `needs_review` | Objective checks passed. Read the session; the model may still have lied, skipped a step, or behaved badly. |
| `fail` | The app exited nonzero or an objective check failed. |
| `blocked` | Lemonade, the selected model, the terminal, or the build prevented a verdict. Not a model result. |
| `interrupted` / `not_run` | The user stopped the batch or this case never started. No verdict. |

Fill each report's `review` fields with `pass`, `fail`, or `capability_gap`, plus
brief notes. Review truthful reporting, current sources, cancellation, and
interaction quality. File checks alone are not a model pass.

The profile is local Lemonade, 8K input budget, 4K maximum output, temperature
0.2, and manual approval. Read/write/execute are enabled; commands still need
consent. Model weights/quantization and the server's loaded context are not
changed by the helper. Record Lemonade/FLM versions and loaded context from
`lemonade status` when reviewing a run.

Fixtures have their own Git root, settings, home and session directory; personal
settings stay separate. Host provider and cloud credentials are removed from the
app and from the independent test subprocess: a case needs the local Lemonade
endpoint, not an inherited hosted key. This is not a Bash sandbox. No dependency
installation, model download, service restart, cloud fallback, or automatic
approval is part of a run.

Evidence stays under ignored `evals/results/` as one batch directory: the
`summary.json`, each case's reports, actual session JSONL files, the fixture, the
app binary hash, Git revision/dirty state, profile, independent test output, and
file-change checks. Keep a failing run until its issue is understood. Delete that
run's directory when the evidence is no longer needed.

Timing fields start as `null`. Time spent entering prompts and granting approvals
is not model latency. The app's request/first-text diagnostics can help inspect
delays, but this helper does not claim backend TTFT or sustained token throughput.

## What we know so far

See [baseline.md](baseline.md) for the evidence already collected. Start with one
run per case; repeat a failed or inconsistent case twice before treating a
prompt-specific result as a model limitation. Avoid an aggregate score that hides
unsupported capabilities. API-level text/tools/vision checks remain in
`examples/lemonade/smoke.py`; a server vision pass does not establish TUI images.

The permission regression reproduces the stale Execute checkbox after a preset
change, then verifies that enabling Execute reaches runtime and disk. A separate
production-loop check runs real Bash only after consent and confirms that
rejection has no side effect. These deterministic checks do not need a model.

Launching and driving another interactive TUI is a separate capability. Shell
permission does not supply a PTY controller or a noninteractive child interface.
The screenshot proves a permission failure, not a system ban on child processes.
Do not copy the repository to test a child: use the existing binary and a fresh
workspace. Until a real child-session trial is recorded, mark that workflow
untested rather than inferring it from Bash or delegated-agent tests.

## Verify the checks themselves

```bash
python -m unittest discover -s evals -p 'test_*.py' -v
go test ./cmd/agent-harness -run 'TestPermissionPresetRefreshes|TestEnabledShell|TestProductionRegistry'
go test ./internal/runtime/tools/builtin -run TestUnavailableSearch
```

The Python tests reject empty implementations, modified test suites, and
out-of-scope files, then cover selection validation, one build per batch, case
isolation, continuation after failure, interruption, blocked preflight, and
prepare-only. Ordinary Go tests continue covering tool validation, bounded
repetition, permissions, and UI state; they are not replaced by live model evals.

## Feedback loop

After a session, inspect its saved report and transcript, then run
`python evals/run.py review <case>/report.json`. Record observed outcomes and
keep suspected causes tentative. The review command never invokes a model or
changes objective checks. A blocked or interrupted run cannot receive a task
verdict.

For a failure, first decide whether it is deterministic. Tool argument
validation, permission state, changed files, and test results should have
focused regression tests. Prompt choice, recovery quality, and interaction
require a live session with the selected model. Fix the diagnosed cause, rerun
the affected check, then try one unfamiliar task. Example: ask it to rename a
small function in a disposable Go package, run the package tests, and preserve
unrelated files. This is a manual generalization check, not a hidden benchmark.

Count another assistant or manual work outside Agent-Harness only when it was
needed to finish the request. Ordinary approvals and shell commands run inside
the app do not count. Record `unknown` when the evidence is unclear.
