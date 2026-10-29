package agent

import (
	"context"
	"fmt"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// prepareRepeatedTools never re-executes a guarded duplicate. Once per query,
// a failed read-only call gets a corrective result so the model can adapt.
// Successful reads, writes, and unknown tools retain the hard stop.
func (l *Loop) prepareRepeatedTools(ctx context.Context, state *loopState, uses []types.ToolUseBlock, defs []tools.Tool, out chan<- types.StreamEvent) ([]types.ToolUseBlock, error) {
	counts := make(map[string]int, len(uses))
	suppressed := make(map[int]bool)
	maxIdentical := l.Config.MaxIdenticalToolUses
	if maxIdentical <= 0 {
		maxIdentical = 1
	}
	for i, use := range uses {
		key := toolCallSignature(use)
		counts[key]++
		count := state.executedTools[key] + counts[key]
		if count <= maxIdentical {
			continue
		}
		def, found := findTool(defs, use.Name)
		if !state.repeatedReadRecoveryUsed && found &&
			def.Capabilities.IsReadOnly != nil && def.Capabilities.IsReadOnly(use.Input) &&
			previousToolFailed(state.messages, key) {
			suppressed[i] = true
			continue
		}
		msg := fmt.Sprintf("[Tool loop detected: %s was called %d times with identical input. The repeated call was not executed.]", use.Name, count)
		select {
		case out <- types.StreamMessage{Message: types.Message{Role: types.RoleSystem, Content: []types.ContentBlock{types.TextBlock{Text: msg}}}}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		// Settle every proposal in the stopped batch. The user can then
		// continue without sending orphaned tool calls back to the provider.
		for _, pending := range uses {
			if err := emitRejectedTool(ctx, state, pending, "The repetition guard stopped this batch. This call was NOT executed. Review the previous tool error and choose a different input or approach.", out); err != nil {
				return nil, err
			}
		}
		return nil, fmt.Errorf("tool repetition limit: %s requested the same input %d times; the repeated call was not executed", use.Name, count)
	}
	for key := range counts {
		counts[key] += state.executedTools[key]
	}
	state.executedTools = counts
	if len(suppressed) == 0 {
		return uses, nil
	}
	state.repeatedReadRecoveryUsed = true
	notice := types.Message{Role: types.RoleSystem, Content: []types.ContentBlock{types.TextBlock{Text: "A repeated failed read was skipped. Asking the agent to correct its input once before stopping."}}}
	select {
	case out <- types.StreamMessage{Message: notice}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	var remaining []types.ToolUseBlock
	for i, use := range uses {
		if !suppressed[i] {
			remaining = append(remaining, use)
			continue
		}
		if err := emitRejectedTool(ctx, state, use, "This identical read-only call already failed. The duplicate was NOT executed. Inspect the previous tool error and change the tool or its arguments. Directory tools accept a literal directory path such as '.', not a shell command or filename. If a requested file is missing, list its parent directory once and use the actual filename. Do not repeat the same call; one correction opportunity remains before the repetition guard stops the turn.", out); err != nil {
			return nil, err
		}
	}
	return remaining, nil
}

func emitRejectedTool(ctx context.Context, state *loopState, use types.ToolUseBlock, reason string, out chan<- types.StreamEvent) error {
	result := types.Message{Role: types.RoleUser, Content: []types.ContentBlock{types.ToolResultBlock{
		ToolUseID: use.ID, IsError: true, Content: reason,
	}}}
	state.messages = append(state.messages, result)
	select {
	case out <- types.StreamMessage{Message: result}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// The last message is the current assistant proposal, not a completed call.
// Walk earlier results/calls backwards to pair the most recent matching call.
func previousToolFailed(messages []types.Message, signature string) bool {
	results := make(map[string]bool)
	for i := len(messages) - 2; i >= 0; i-- {
		for j := len(messages[i].Content) - 1; j >= 0; j-- {
			switch block := messages[i].Content[j].(type) {
			case types.ToolResultBlock:
				if _, seen := results[block.ToolUseID]; !seen {
					results[block.ToolUseID] = block.IsError
				}
			case types.ToolUseBlock:
				if toolCallSignature(block) == signature {
					return results[block.ID]
				}
			}
		}
	}
	return false
}
