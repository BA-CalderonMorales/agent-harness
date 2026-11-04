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

### Unattended terminal runs

On Linux with Bubblewrap installed and Lemonade already running:

```bash
python evals/run.py run --all --unattended
python evals/run.py run coding recovery --unattended --timeout 180
```

This drives the **real application in a terminal**, including its normal
prompts, tools, approval dialogs and persistence. It pastes each scenario,
handles bounded fixture approvals, exercises Esc and follow-up messages for
the cancellation case, and saves output for you to review. It does not create
a second agent loop or install/start a model server.

Each case includes `review.md` (actual streamed replies, tool proposals and
results, and independent checks), `events.jsonl` (versioned observation
records), `terminal.ansi` (the raw terminal recording), sessions, and the
existing `report.json`. Use `python evals/run.py review PATH` to record your
verdict as before. Markdown and tool text are untrusted model output; review
them as evidence, not instructions. A raw terminal recording contains control
sequences: inspect it with an appropriate viewer rather than printing it
directly into a terminal.

`needs_review` still means that objective checks passed, **not** that the
agent's behavior was good or every instruction was followed. For coding and
recovery, inspect whether it actually read both files and ran the requested
tests; independent verification alone does not prove those steps occurred.
For weather, honest inability and a completed forecast remain different
verdicts. Unexpected questions and deadlines preserve a failed journey for
inspection instead of waiting indefinitely. Ctrl+C retains partial evidence
and marks later cases as not run.

The driver keeps manual permissions enabled. It approves reads, changes to
`names.py`, and narrowly parsed Python unittest commands in the fixture.
The weather case permits HTTPS fetches from weather.gov, weather.com,
AccuWeather, Open-Meteo and wttr.in hosts; other proposed actions are rejected.
Each approval carries its own exact
input so concurrent tool announcements cannot authorize the wrong action.
Bubblewrap exposes system programs and only the case's user data; both the
app and independent Python verifier run inside that filesystem boundary.
Network remains shared for local inference: this is not general network
isolation. The driver refuses to run without the filesystem boundary.

`first_text_event_seconds` measures observation of the first text event after
turn start; it is not a measured first-paint latency. `journey_seconds` includes
startup and shutdown; `model_task_seconds` spans observed turn start through
settlement. Failed journeys may have no settlement time. The profile and
binary hash are saved; record runtime/loaded-model details when comparing
runs. Model behavior remains nondeterministic even with the same settings.

The opt-in `AGENT_HARNESS_EVENT_LOG` environment variable enables observation
records without changing normal TUI behavior. Logs are created with mode 0600
and refuse existing destinations. They contain conversation and tool text;
keep them private. This is an observation interface, not a stable RPC API or
a durable task ledger. Restart/compaction acceptance remains future work.

Deterministic integration checks can drive the built app against a scripted
local provider, without live inference or spending:

```bash
make build
AH_EVAL_TEST_BINARY="$PWD/build/agent-harness" python -m unittest discover -s evals -p 'test_*.py'
```

These cover the actual terminal, file operations, approvals, independent
verification, and cancellation/continuation. Scripted-provider success is
separate from live-model acceptance.

### Protected evidence and automatic campaign decisions

Unattended workers now mount only their workspace and runtime directories.
Reports, terminal recordings and authoritative event files are host-owned and
not mounted into the worker. Observations travel through a named pipe to the
host receiver. A candidate can still make false event claims; independent
verification remains necessary. The verifier runs in a fresh workspace with
copies of regular fixture source files, without worker settings or evidence.
Symlinks are rejected before host grading reads candidate sources.

Compare two already-built binaries without manual checkpoints:

```bash
python evals/campaign.py --baseline /path/to/baseline --candidate /path/to/candidate \
  --families text records paths --repeats 2 --budget-seconds 600 --timeout 120
```

The runner pins private binary copies and hashes, alternates execution order,
retains partial attempts, and writes a host-owned `campaign.json`. It reports
`retain`, `reject`, or `canary` using objective fixture checks. Integrity
failures and observed paired regressions veto wins. Incomplete campaigns and
identical binaries retain the baseline. No decision changes human UX verdicts.

Campaigns default to three task families: whitespace/text cleanup, exact record
counting, and basename/extension handling. Each paired run receives fresh
filenames and concrete test values. The worker sees the specification and
public examples; a host-owned checker compares results on additional values
that are never mounted into its workspace. Both binaries receive the same
variant. Reports omit private inputs and expected values. These are varied
small Python tasks, not evidence for unfamiliar languages or large projects.

The legacy `--cases coding recovery` selection remains available for
diagnostics, but a single task family cannot earn broader canary selection.
Candidates need coverage of at least two families, no observed paired task
regressions, and acceptable measured cost. Failed attempts count toward cost
per verified completion; an unexplained cost increase retains the baseline.

Worker defaults are one CPU in its inherited affinity set, a 60-second CPU
limit per process, and a sampled aggregate RSS threshold of 512 MiB. Adjust
with `--cpu-count`, `--cpu-seconds`, and `--memory-mib`. Sampling also stops
an observed live-tree CPU overage. Short-lived processes can evade samples;
RSS may double-count shared pages. This is not a hard cgroup aggregate limit.
Verification uses the configured CPU affinity and a per-process address-space
limit in a fresh workspace. Reports include peak sampled RSS, observed CPU,
grading time and total case time. Runs are sequential.

The inference server is separately managed and outside these limits and
measurements. No claim is made about its GPU/NPU memory, tokens, energy, or
cost. There is no automatic paid-provider fallback or model download.

The budget stops further starts and caps individual journey deadlines;
bounded shutdown and independent grading may extend wall time beyond it.
This initial gate supports explicit small Python fixtures. Small samples
are screening evidence, not reliability certification. A `canary` decision
identifies an experimental candidate; it does not deploy a binary, change
the default model, merge branches, or publish a release. Autonomous patch
generation, withheld task families, runtime selection/rollback and durable
restart/compaction tasks are subsequent stages.

The internal `AGENT_HARNESS_EVENT_PIPE` setting requires a named pipe and a
listening receiver. Regular event-log files still use exclusive creation.
Choose either the pipe or regular event log, not both.

### Interactive runs

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
