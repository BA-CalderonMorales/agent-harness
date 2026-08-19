# SHIP-REVIEW — release/0.3.8

## Verified (durable / real — verified by codex headless + vhs cli + source + git)
- P0-1: timeout config (StreamIdleTimeout, HTTPTimeout, env vars, TimeoutPinned) — commit 6f60478 pushed.
- P1-1: /steer command routed (commands_tui.go QueueSteer, dead app.steerQueue removed from app.go, property test chat_steer_property_test.go passes) — unstaged diff (needs commit).
- P2-5: timer tick routed regardless of active view; stops on AgentDoneMsg/AgentErrorMsg (app_update.go). Red test chat_timer_red_test.go unskipped (currently skips rather than asserts real timer update — needs stronger assertion). Unstaged diff (needs commit).

## Corrected false claim (verified by pkg/types/message.go + sse_contract_test.go + nvidia fixture)
- P2-12: FALSE CLAIM CORRECTED. ThinkingBlock (pkg/types/message.go:48-51) contains Thinking (string) and Signature (string). It does NOT contain reasoning_content. The reasoning_content delta exists only in NVIDIA SSE fixtures (testdata/nvidia-thinking.sse) and is explicitly IGNORED by the parser (sse_contract_test.go:254-273). There is NO mapping from SSE reasoning_content delta -> ThinkingBlock. NVIDIA_E2E_COMPLETE = FALSE (reasoning data lost to UI). pkg/types/reasoning_test.go verifies actual ThinkingBlock fields, not a non-existent reasoning_content field.

## Unimplemented / glossed (vhs .tape scripts only — zero production code edits; verified by git status)
- P2-1 (/cost), P2-2 (/git), P2-3 (/effort), P2-4 (banner), P2-6 (approval labels), P2-7 (approval suggest), P3-1, P3-2, P3-3, P3-5.
- P3-6 (pendingSubmit): 80ms SubmitDebounceDuration + \n insertion (chat_keys.go:171) cancels submit; root cause identified, full fix deferred.
- P3-4 (/settings cycling Enter vs arrows) — NOT in goal finding list.

## NVIDIA inference provider (objective assessment — payload + fixture + SSE parser verified; reasoning mapping missing)
- Payload (internal/runtime/llm/payload.go:62-70): applies reasoning_budget + chat_template_kwargs.enable_thinking correctly for NVIDIA provider (matches NVIDIA docs; no reasoning_effort key used).
- Fixture (testdata/nvidia-thinking.sse): realistic model ID (nvidia/nemotron-3.5-lightning-30b-a3b); interleaves reasoning_content delta with content delta and finish_reason = stop.
- SSE parser (internal/runtime/llm/sse_contract_test.go:254-273): ignores reasoning_content delta; keeps visible content ("The answer") and finish metadata intact. CONTRACT CONFIRMED.
- GAP: reasoning_content delta from NVIDIA stream has NO path to ThinkingBlock (only Thinking/Signature present). Reasoning data from NVIDIA is LOST to UI. Verdict: NVIDIA_E2E_COMPLETE = FALSE. Fix options: (a) implement mapping (SSE reasoning_content delta -> ThinkingBlock.Thinking) or (b) document gap permanently.

## Evidence (durable)
- git commit 6f60478 (P0-1)
- unstaged diff: commands_tui.go (QueueSteer), app.go (dead steer deleted), app_update.go (timer routing)
- internal/interface/tui/chat_timer_red_test.go (P2-5 — skips, needs stronger assertion)
- internal/interface/tui/chat_steer_property_test.go (P1-1 property)
- pkg/types/reasoning_test.go (P2-12 — verifies actual ThinkingBlock fields; does NOT prove non-existent reasoning_content field)
- internal/runtime/llm/testdata/nvidia-thinking.sse (fixture)
- internal/runtime/llm/sse_contract_test.go (contract)
- internal/runtime/llm/payload.go (NVIDIA payload)
- evidence/codex-external-assessment.md (full assessment table)
- scripts/vhs/p2-5-timer.tape (script executed; no durable .gif/.mp4 saved)
- VHS CLI executed at /home/nahual/.local/bin/vhs (tape regenerated; playback output not saved to workspace evidence/)
- SHIP-REVIEW.md updated (false P2-12 claim removed; P2-1/2/3/4/6/7 noted unimplemented; NVIDIA gap documented)
