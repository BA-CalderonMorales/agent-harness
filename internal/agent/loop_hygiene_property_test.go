package agent

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// queryOnce drives one full loop turn against the mock and drains the
// stream to completion.
func queryOnce(t *testing.T, loop *Loop) {
	t.Helper()
	params := QueryParams{
		Messages: []types.Message{},
		CanUseTool: func(string, map[string]any, tools.Context) (tools.PermissionDecision, error) {
			return tools.PermissionDecision{Behavior: tools.Allow}, nil
		},
		ToolUseContext: tools.Context{AbortController: context.Background()},
	}
	stream, err := loop.Query(context.Background(), params)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	for range stream { // drain: the stream closes when the turn ends
	}
}

// settleGoroutines waits briefly for the scheduler to retire the
// turn's goroutines, then reports the count. A bounded retry keeps the
// property deterministic without sleeping a fixed wall clock.
func settleGoroutines(ceiling int) int {
	deadline := time.Now().Add(2 * time.Second)
	best := runtime.NumGoroutine()
	for time.Now().Before(deadline) {
		best = runtime.NumGoroutine()
		if best <= ceiling {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	return best
}

// Property: the loop leaks no goroutines across turns. Every
// blocked-leaked goroutine pins an OS thread, and thread exhaustion
// (errno=11, newosproc) takes down unrelated processes on the box —
// the linker itself died that way during `make run`. N sequential
// turns must return the goroutine count to baseline.
func TestLoopGoroutineHygieneProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	properties := gopter.NewProperties(parameters)

	properties.Property("N turns return the goroutine count to baseline", prop.ForAll(
		func(n int) string {
			baseline := runtime.NumGoroutine()
			loop := NewLoop(&llm.MockClient{Events: llm.MockTextResponse("ok")})
			for i := 0; i < n; i++ {
				queryOnce(t, loop)
			}
			// +2 tolerance: GC and finalizer helpers come and go.
			if got := settleGoroutines(baseline + 2); got > baseline+2 {
				return fmt.Sprintf("%d turns: goroutines %d exceed baseline %d — a turn leaked", n, got, baseline)
			}
			return ""
		},
		gen.IntRange(1, 12),
	))

	properties.TestingRun(t)
}
