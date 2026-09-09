package agent

import (
	"fmt"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
)

func TestLoopCompletesDevelopmentWorkflowWithoutContinuation(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(fmt.Sprintf("streaming=%v", streaming), func(t *testing.T) {
			var calls int
			tool := edgeTool(&calls)
			client := &scriptedLoopClient{}
			// Explore, verify, repair, and verify again: exceeds both old defaults
			// and repeats a command after intervening work.
			commands := []string{}
			for i := 0; i < 16; i++ {
				commands = append(commands, fmt.Sprintf("read file %d", i))
			}
			commands = append(commands, "go test ./...", "edit implementation", "go test ./...")
			for i, command := range commands {
				client.streams = append(client.streams, toolEvents(fmt.Sprintf("tool_%d", i), tool.Name, fmt.Sprintf(`{"cmd":%q}`, command)))
			}
			client.streams = append(client.streams, textEvents("Implemented and verified."))
			loop := NewLoop(client)
			loop.Config.StreamingToolExecution = streaming
			terminal, _ := runLoopForEdgeTest(t, loop, edgeParams([]tools.Tool{tool}), 512)
			if terminal.Reason != TerminalReasonComplete || calls != len(commands) {
				t.Fatalf("reason=%s executions=%d, want complete and %d executions", terminal.Reason, calls, len(commands))
			}
			if client.callCount() != len(commands)+1 {
				t.Fatal("final verification result was not consumed")
			}
		})
	}
}

func TestLoopAlternatingCallsRemainBounded(t *testing.T) {
	var calls int
	tool := edgeTool(&calls)
	client := &scriptedLoopClient{}
	for i := 0; i < 5; i++ {
		client.streams = append(client.streams, toolEvents(fmt.Sprintf("tool_%d", i), tool.Name, fmt.Sprintf(`{"cmd":"read %d"}`, i%2)))
	}
	loop := NewLoop(client)
	params := edgeParams([]tools.Tool{tool})
	params.MaxToolCalls = 4
	terminal, _ := runLoopForEdgeTest(t, loop, params, 128)
	if terminal.Reason != TerminalReasonBlockingLimit || calls != 4 {
		t.Fatalf("reason=%s executions=%d, want blocking_limit after 4", terminal.Reason, calls)
	}
}
