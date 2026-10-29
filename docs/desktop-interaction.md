# Desktop interaction contract

Desktop work must feel predictable before the same session controls reach
a phone or tablet. Evaluate the complete conversation, including errors,
rather than judging readiness from a successful completion request.

## Reference behavior and boundaries

Use Pi and OpenCode as primary references for observable workflows.
Crush, Copilot, Claude Code, and Droid are additional comparison points
for request intake, concise updates, approvals, and continuation. This is
a behavioral comparison, not a claim that their internal designs or
permission defaults should be copied.

Two patterns verified in Pi's source are useful here:

- Its [tool lifecycle](https://github.com/earendil-works/pi/blob/main/packages/agent/src/agent-loop.ts)
  validates arguments and carries execution failures into tool results and
  completion events. The UI and next model turn can consume the same outcome.
- Its [skill prompt](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/src/core/skills.ts)
  lists descriptions and file locations, with instructions to load a skill
  when relevant. Agent-Harness currently injects full skill bodies.

OpenCode documents separate [tools](https://opencode.ai/docs/tools/) and
[permission decisions](https://opencode.ai/docs/permissions/). Keep the
distinction between a proposed action, permission to run it, and its actual
result visible in our own event flow.

## Acceptance journeys

| Journey | Required behavior | Evidence |
|---|---|---|
| Standalone greeting | Brief response without unnecessary tools or skill bodies | Measure first visible answer and full request size; pending |
| Read/edit/test | Inspect actual files, change only the requested implementation, run verification, summarize the observed result | Disposable names.py fixture; real-model completion pending |
| Typo in a filename | Inspect the parent directory, identify the real file, preserve the requested task | Scripted loop regression covers corrected discovery after failed listing |
| Wrong tool arguments | Explain the expected shape to the model; do not silently interpret directory paths as shell commands | Directory-tool error tests |
| Repeated failed read | Suppress the duplicate and allow one correction opportunity; stop persistent repetition | Streaming and batch regression tests |
| Repeated write | Never automatically replay a failed write under the read-recovery policy | Regression tests count actual executions |
| Missing mutation arguments | Reject missing/null/non-string content or replacement text before touching disk, including after a policy rewrite | File-tool and executor regression tests; explicit empty strings remain valid |
| Approval or denial | Show the exact proposed action and actual decision; preserve existing policy enforcement | Existing approval contracts; include in live comparison |
| Cancellation and resume | Settle pending rows truthfully and retain valid conversation history | Existing lifecycle tests plus duplicate-result pairing tests; live check pending |
| Provider unavailable | Identify endpoint/model/authentication failures separately from agent work limits | Provider feedback tests |

A missing file cannot appear as a successful directory operation. A
directory listing cannot appear as shell execution. A repetition stop
cannot default to advice about an unresponsive model.

## What the recovery patch changes

Directory-listing failures now return execution errors with a literal-path
example. An identical failed read-only proposal is rejected without
execution and returned as a corrective tool result once per query. The
next model turn can choose a valid tool or input. A persistent duplicate
still stops, with every proposal in the stopped batch given a rejected
result so a later continuation has no orphaned tool calls.

This is bounded recovery, not an automatic filename substitution or a
shell-command repair. It does not bypass permissions, rerun writes, or
guarantee that a small model will choose the right next action.

The subsequent 2B live attempt reached edits and test commands, but changed
the excluded test file and sent edit-shaped arguments to write without
the required content. The old accessor silently turned that omission into
an empty string and truncated names.py. That run fails acceptance. Both
fixture files were backed up and restored to their original Git baseline.
The mutation guard now distinguishes missing values from intentional
empty strings. Prompt instructions about task scope alone are not an
enforced filesystem policy; do not claim otherwise.

## Comparison protocol

Run the same disposable fixture through Agent-Harness, Pi, and OpenCode
using the same model and backend first. Record system/tool prompt sizes,
context, thinking settings, runtime version, task completion, tool errors,
human interventions, changed files, and time to verified completion.
Then compare a stronger model separately. Hosted flagship responses are
not evidence that an identical interaction will work on a local 2B model.

Judge narration at three moments: initial understanding, a meaningful
decision or recovery, and the final verified outcome. Repeated promises
to start work do not count as progress. Never report tests as passing
without execution evidence.

## Next implementation priorities

1. Reduce prompt overhead through skill discovery and on-demand loading.
2. Provide precise tool descriptions and schema validation, including
   examples that distinguish literal paths from shell commands.
3. Measure first visible response, reasoning, execution, and total duration
   separately. Compare cold and warm turns.
4. Exercise cancellation, steering, approvals, and resume on the real
   desktop interface with the selected local model.
5. Carry the same task/result/approval contracts into a future mobile
   client after desktop acceptance; choose touch interaction separately.
