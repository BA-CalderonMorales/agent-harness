package agent

import (
	"context"
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"time"
)

func findTool(defs []tools.Tool, name string) (tools.Tool, bool) {
	for _, t := range defs {
		if t.Name == name {
			return t, true
		}
		for _, a := range t.Aliases {
			if a == name {
				return t, true
			}
		}
	}
	return tools.Tool{}, false
}

func isBashTool(name string) bool {
	return name == "bash" || name == "BashTool"
}

// runSingleTool executes one tool call.
func runSingleTool(ctx tools.Context, block types.ToolUseBlock, assistantMsg types.Message, defs []tools.Tool, canUseTool tools.CanUseToolFn, onProgress tools.OnProgress) (types.Message, error) {
	toolDef, toolFound := findTool(defs, block.Name)

	// Work on a private input map so canonicalization hooks cannot mutate the
	// assistant message or another policy layer's view of the proposal.
	input := make(map[string]any, len(block.Input))
	for key, value := range block.Input {
		input[key] = value
	}

	var validationErr error
	if toolFound && toolDef.ValidateInput != nil {
		vr := toolDef.ValidateInput(block.Input, ctx)
		if !vr.Valid {
			validationErr = fmt.Errorf("validation failed: %s", vr.Message)
		}
	}

	// Canonicalize before either policy layer sees the proposal.
	if toolFound && toolDef.BackfillObservableInput != nil {
		toolDef.BackfillObservableInput(input)
	}

	globalDecision := tools.PermissionDecision{
		Behavior:     tools.Allow,
		UpdatedInput: input,
	}
	if canUseTool != nil {
		var err error
		globalDecision, err = canUseTool(block.Name, input, ctx)
		if err != nil {
			return types.Message{}, fmt.Errorf("permission check error: %w", err)
		}
	} else if ctx.RequireCanUseTool {
		return types.Message{}, fmt.Errorf("permission denied: global policy is unavailable")
	}

	if globalDecision.UpdatedInput != nil {
		input = globalDecision.UpdatedInput
	}

	auditEvent := func(event string, behavior tools.DecisionBehavior, message string, durationMillis int64, eventErr error) error {
		if globalDecision.Audit == nil {
			return nil
		}
		return globalDecision.Audit(tools.ToolAuditEvent{
			Event:          event,
			ToolCallID:     block.ID,
			ToolName:       block.Name,
			Input:          input,
			Behavior:       behavior,
			Message:        message,
			DurationMillis: durationMillis,
			Err:            eventErr,
		})
	}

	if err := auditEvent("proposal", globalDecision.Behavior, globalDecision.Message, 0, nil); err != nil {
		return types.Message{}, fmt.Errorf("permission denied: durable proposal audit failed: %w", err)
	}

	if !toolFound {
		decisionErr := fmt.Errorf("Error: No such tool available: %s", block.Name)
		if err := auditEvent("decision", tools.Deny, decisionErr.Error(), 0, decisionErr); err != nil {
			return types.Message{}, fmt.Errorf("%v (decision audit failed: %w)", decisionErr, err)
		}
		return types.Message{}, decisionErr
	}

	if validationErr != nil {
		if err := auditEvent("decision", tools.Deny, validationErr.Error(), 0, validationErr); err != nil {
			return types.Message{}, fmt.Errorf("%v (decision audit failed: %w)", validationErr, err)
		}
		return types.Message{}, validationErr
	}

	localDecision := tools.PermissionDecision{
		Behavior:     tools.Allow,
		UpdatedInput: input,
	}
	if toolDef.CheckPermissions != nil {
		localDecision = toolDef.CheckPermissions(input, ctx)
	}
	decision := mergePermissionDecisions(globalDecision, localDecision)
	if decision.UpdatedInput != nil {
		input = decision.UpdatedInput
	}

	if decision.Behavior == tools.Deny {
		decisionErr := fmt.Errorf("permission denied: %s", decision.Message)
		if err := auditEvent("decision", tools.Deny, decision.Message, 0, decisionErr); err != nil {
			return types.Message{}, fmt.Errorf("%v (decision audit failed: %w)", decisionErr, err)
		}
		return types.Message{}, decisionErr
	}

	if decision.Behavior == tools.Ask {
		if globalDecision.Checkpoint == nil {
			decisionErr := fmt.Errorf("permission denied: approval checkpoint is unavailable")
			if err := auditEvent("decision", tools.Deny, decisionErr.Error(), 0, decisionErr); err != nil {
				return types.Message{}, fmt.Errorf("%v (decision audit failed: %w)", decisionErr, err)
			}
			return types.Message{}, decisionErr
		}

		checkpointDecision, err := globalDecision.Checkpoint()
		if err != nil {
			decisionErr := fmt.Errorf("approval checkpoint failed: %w", err)
			if auditErr := auditEvent("decision", tools.Deny, decisionErr.Error(), 0, decisionErr); auditErr != nil {
				return types.Message{}, fmt.Errorf("%v (decision audit failed: %w)", decisionErr, auditErr)
			}
			return types.Message{}, decisionErr
		}
		if checkpointDecision.UpdatedInput != nil {
			input = checkpointDecision.UpdatedInput
		}
		if checkpointDecision.Behavior != tools.Allow {
			decisionErr := fmt.Errorf("permission denied: %s", checkpointDecision.Message)
			if auditErr := auditEvent("decision", tools.Deny, checkpointDecision.Message, 0, decisionErr); auditErr != nil {
				return types.Message{}, fmt.Errorf("%v (decision audit failed: %w)", decisionErr, auditErr)
			}
			return types.Message{}, decisionErr
		}
		decision.Behavior = tools.Allow
		decision.Message = checkpointDecision.Message
	}

	if err := auditEvent("decision", tools.Allow, decision.Message, 0, nil); err != nil {
		return types.Message{}, fmt.Errorf("permission denied: durable decision audit failed: %w", err)
	}

	started := time.Now()
	if err := auditEvent("start", tools.Allow, "", 0, nil); err != nil {
		return types.Message{}, fmt.Errorf("tool execution blocked: durable start audit failed: %w", err)
	}

	result, err := toolDef.Call(input, ctx, canUseTool, onProgress)
	if err != nil {
		durationMillis := time.Since(started).Milliseconds()
		if auditErr := auditEvent("failure", tools.Allow, err.Error(), durationMillis, err); auditErr != nil {
			return types.Message{}, fmt.Errorf("tool failed: %v (outcome audit failed: %w)", err, auditErr)
		}
		return types.Message{}, err
	}

	// Admit the full result when it fits. Otherwise preserve the exact output
	// out of band and admit only a bounded, retrievable receipt.
	budget := tools.GetCurrentBudget()
	resultStr := fmt.Sprintf("%v", result.Data)
	receiptLimit, admitted := budget.TryRecordResult(block.Name, len(resultStr), toolDef.MaxResultSizeChars)
	for !admitted {
		receipt, receiptErr := persistToolResultReceipt(resultStr, receiptLimit)
		if receiptErr != nil {
			resultErr := fmt.Errorf("preserve oversized tool result: %w", receiptErr)
			durationMillis := time.Since(started).Milliseconds()
			if auditErr := auditEvent("failure", tools.Allow, resultErr.Error(), durationMillis, resultErr); auditErr != nil {
				return types.Message{}, fmt.Errorf("tool result handling failed: %v (outcome audit failed: %w)", resultErr, auditErr)
			}
			return types.Message{}, resultErr
		}

		nextLimit, receiptAdmitted := budget.TryRecordResult(block.Name, len(receipt), toolDef.MaxResultSizeChars)
		if receiptAdmitted {
			result.Data = receipt
			admitted = true
			break
		}
		if nextLimit >= receiptLimit {
			resultErr := fmt.Errorf(
				"preserve oversized tool result: available budget did not shrink after receipt admission failed",
			)
			durationMillis := time.Since(started).Milliseconds()
			if auditErr := auditEvent("failure", tools.Allow, resultErr.Error(), durationMillis, resultErr); auditErr != nil {
				return types.Message{}, fmt.Errorf("tool result handling failed: %v (outcome audit failed: %w)", resultErr, auditErr)
			}
			return types.Message{}, resultErr
		}
		receiptLimit = nextLimit
	}

	mapped := toolDef.MapResult(result.Data, block.ID)
	if err := auditEvent("success", tools.Allow, "", time.Since(started).Milliseconds(), nil); err != nil {
		return types.Message{}, fmt.Errorf("tool succeeded but outcome audit failed: %w", err)
	}
	return types.Message{
		Role:    types.RoleUser,
		Content: []types.ContentBlock{mapped},
	}, nil
}

func mergePermissionDecisions(global, local tools.PermissionDecision) tools.PermissionDecision {
	merged := global
	if local.UpdatedInput != nil {
		merged.UpdatedInput = local.UpdatedInput
	}

	if global.Behavior == tools.Deny {
		return merged
	}
	if global.Behavior != tools.Allow && global.Behavior != tools.Ask {
		merged.Behavior = tools.Deny
		merged.Message = "unsupported global permission decision"
		return merged
	}

	switch local.Behavior {
	case tools.Deny:
		merged.Behavior = tools.Deny
		merged.Message = local.Message
	case tools.Ask:
		merged.Behavior = tools.Ask
		if local.Message != "" {
			merged.Message = local.Message
		}
	case tools.Allow:
		if global.Behavior == tools.Ask {
			merged.Behavior = tools.Ask
		} else {
			merged.Behavior = tools.Allow
		}
	default:
		merged.Behavior = tools.Deny
		merged.Message = "unsupported tool-specific permission decision"
	}
	return merged
}

// runToolsBatch executes a batch of tools with partitioning.
func runToolsBatch(ctx context.Context, blocks []types.ToolUseBlock, assistantMsg types.Message, toolCtx tools.Context, canUseTool tools.CanUseToolFn) ([]types.Message, error) {
	var out []types.Message
	for _, block := range blocks {
		msg, err := runSingleTool(toolCtx, block, assistantMsg, toolCtx.Options.Tools, canUseTool, nil)
		if err != nil {
			msg = types.Message{
				Role:    types.RoleUser,
				Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: block.ID, Content: err.Error(), IsError: true}},
			}
		}
		out = append(out, msg)
	}
	return out, nil
}
