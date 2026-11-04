# Qwen3.5-4B starting evidence

Recorded 2026-10-29. This is prior user-run evidence and a code audit, not a
new automated live-model run. The model is `qwen3.5-4b-FLM` through Lemonade
on the ZBook NPU. The reported server version is 2026.42.1 and FLM is 1.0.9;
the earlier loaded context was 16,384. Re-record these for new runs.

| Capability | Evidence | Assessment |
| --- | --- | --- |
| Server text/tool call/tool result/vision | User-run smoke: 1.24s / 1.97s / 1.51s / 1.40s; all passed | Demonstrated for those synthetic requests |
| Small coding task | Session read both files, made one write, ran four tests, and reported success; independent test discovery passed and only `names.py` changed | One successful task; model enumerated tests rather than issuing the exact requested command |
| Perceived coding speed | User described the successful run as surprisingly fast | Positive UX evidence; no new measured latency baseline |
| Missing-file recovery | Prior failures motivated bounded-retry fixes; no repeated 4B live trial recorded | Untested with the current build |
| Omaha weather | User screenshot shows placeholder search results and no forecast; retry took 30.6s according to the UI | Task failed; current web search was not implemented, so this is a harness capability gap rather than proof of model inability |
| Consented Bash | Actual log: `bash — BASH permission disabled`; saved `perm_execute` was false despite the reported enabled UI | Permission/UI mismatch; not an OS or model capability conclusion |
| Child app instance | Screenshot's attempted shell command was denied before execution | Not demonstrated; copying the repo and driving a child TUI were not validated |
| Cancel and continue | Deterministic UI regression tests pass | Live 4B interaction remains untested |

The permission bug has a deterministic reproduction: changing a permission preset
resets Execute in the runtime without refreshing the visible checkbox. Settings
now receives the authoritative snapshot after changes. Explicit deny rules and
permission-off behavior are retained; approval/rejection is tested with actual
Bash side effects in a temporary workspace.

The live `web_search` implementation discarded its HTTP response and returned
placeholder JSON as a success. It is now excluded from enabled tools, and stale
calls return an explicit unavailable error without making a request. This fixes
false capability advertising; it does not implement search. `web_fetch` remains
available for known source URLs, subject to its existing network policy.

The local cost figure visible in the screenshot is not a measured local inference
expense: unknown model names fall through to generic cloud pricing in the cost
tracker. Do not use that dollar figure as a baseline metric. Energy and hardware
costs were not measured.

A fresh coding eval attempted from the restricted agent session was blocked by
inability to connect to the host's `127.0.0.1:13305`. The helper recorded `blocked`
with no model verdict. Run the five cases on the host before promoting this
starting evidence into a current-build baseline. No claim of general coding,
current-information, cancellation, or child-process reliability follows from the
single successful coding task.

## Automated terminal development trials, 2026-11-04

The release/0.4.0 working tree now supports unattended **real-TUI** runs with
`python evals/run.py run --all --unattended`. The connection to local Lemonade
was accessible for these trials. All inference used `qwen3.5-4b-FLM`; no hosted
fallback was used. Runtime version and loaded context were not independently
re-recorded. Runs used the eval profile and retain binary hashes in their
batch summaries. These development trials span multiple dirty builds and
driver corrections; they are not a controlled before/after benchmark.

- A greeting completed without tools, with its actual reply retained.
- A coding trial read both files, wrote only `names.py`, ran the four tests,
  and passed independent verification. Earlier coding failures are also
  retained; one success does not establish reliable coding capability.
- Recovery trials exposed repeated ineffective changes and a deadline. An
  earlier driver incorrectly rejected qualified unittest selections; that
  policy issue was corrected rather than attributed to model ability.
- A real Esc cancellation followed by `READY_TO_CONTINUE` completed. This is
  evidence for that interaction, not restart or compaction acceptance.
- An initial weather trial had a driver-created certificate-store failure.
  The isolated runtime now includes the system certificate targets, with a
  deterministic check that roots remain available. Earlier weather failures
  must not be treated as clean model comparisons.

The traces also exposed a product bug: nonzero Bash exits were delivered as
successful tool results even when the output showed failing tests. Shell
startup errors, nonzero exits and timeouts now carry `IsError`; a real-terminal
scripted-provider check asserts the visible tool-error result.

Actual replies, proposed tool inputs, results, approval decisions, raw terminal
recordings and independent checks remain under ignored `evals/results/`.
Every live report's human-review fields remain unreviewed. Review the output
before assigning a task, honesty, or interaction verdict. These trials do not
establish long-horizon reliability or parity with another coding agent.
