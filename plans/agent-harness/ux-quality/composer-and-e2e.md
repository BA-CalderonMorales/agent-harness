# Composer And E2E Quality Slice

## UX Invariants

- The composer is part of the main chat render and appears in the final lines of the view.
- The input area reserves height from the actual visible textarea rows, not the maximum row count.
- Empty and single-line input reserve one row; multi-line input grows to four rows and then caps.
- Tool messages are not replaced by later assistant text.

## Local Reliability Invariants

- `AGENT_HARNESS_SESSION_DIR` isolates tests and local harness runs.
- Resuming with no sessions returns `(nil, false)` and does not create state.
- Non-session files in the session directory do not prevent resuming a valid session.

## API-Backed E2E Gate

`TestOpenRouterLiveStreamSmoke` is skipped unless `AH_E2E_OPENROUTER=1`.
It accepts `AH_API_KEY`, `AGENT_HARNESS_API_KEY`, or `OPENROUTER_API_KEY`.
Use `AH_E2E_OPENROUTER_MODEL` to override the default live model.
