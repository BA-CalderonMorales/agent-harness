// Package buckets provides domain-specific LoopBase implementations.
package buckets

import (
	"fmt"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/loop"
	"github.com/BA-CalderonMorales/agent-harness/internal/loop/buckets/defaults"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// PlanState tracks the current plan mode state.
// This is package-level for simplicity - in production would be session-scoped.
var planState = &PlanState{}

// PlanState tracks planning session.
type PlanState struct {
	IsActive    bool
	Plan        []PlanStep
	StartTime   time.Time
	CurrentStep int
}

// PlanStep represents a single step in a plan.
type PlanStep struct {
	Number      int
	Description string
	Status      string // pending, active, done, error
	Result      string
}

// LoopPlan handles plan mode operations.
// Tools: enter_plan_mode, exit_plan_mode
type PlanBucket struct {
	onPlanStart func(plan string)
	onPlanStep  func(step int, total int, description string)
	onPlanEnd   func(completed bool)
}

// NewLoopPlan creates a plan bucket.
func Plan() *PlanBucket {
	return &PlanBucket{}
}

// WithCallbacks sets event handlers.
func (p *PlanBucket) WithCallbacks(
	onStart func(plan string),
	onStep func(step int, total int, description string),
	onEnd func(completed bool),
) *PlanBucket {
	p.onPlanStart = onStart
	p.onPlanStep = onStep
	p.onPlanEnd = onEnd
	return p
}

// Name returns the bucket identifier.
func (p *PlanBucket) Name() string {
	return "plan"
}

// CanHandle determines if this bucket handles the tool.
func (p *PlanBucket) CanHandle(toolName string, input map[string]any) bool {
	switch toolName {
	case "enter_plan_mode", "plan", "exit_plan_mode", "approve_plan", "reject_plan":
		return true
	}
	return false
}

// Capabilities describes what this bucket can do.
func (p *PlanBucket) Capabilities() loop.BucketCapabilities {
	return loop.BucketCapabilities{
		IsConcurrencySafe: true,
		IsReadOnly:        false,
		IsDestructive:     false,
		ToolNames:         []string{"enter_plan_mode", "plan", "exit_plan_mode", "approve_plan", "reject_plan"},
		Category:          "plan",
	}
}

// Execute runs the plan operation.
func (p *PlanBucket) Execute(ctx loop.ExecutionContext) loop.LoopResult {
	switch ctx.ToolName {
	case "enter_plan_mode", "plan":
		return p.handleEnterPlan(ctx)
	case "exit_plan_mode":
		return p.handleExitPlan(ctx)
	case "approve_plan":
		return p.handleApprovePlan(ctx)
	case "reject_plan":
		return p.handleRejectPlan(ctx)
	default:
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("unknown_tool", "plan bucket doesn't handle: "+ctx.ToolName),
		}
	}
}

// handleEnterPlan enters plan mode with a proposed plan.
func (p *PlanBucket) handleEnterPlan(ctx loop.ExecutionContext) loop.LoopResult {
	plan, _ := ctx.Input["plan"].(string)
	steps, _ := ctx.Input["steps"].([]any)

	// Limit steps to max
	if len(steps) > defaults.PlanMaxStepsDefault {
		steps = steps[:defaults.PlanMaxStepsDefault]
	}

	planState.IsActive = true
	planState.StartTime = time.Now()
	planState.CurrentStep = 0
	planState.Plan = []PlanStep{}

	// Parse steps if provided
	if len(steps) > 0 {
		for i, s := range steps {
			if stepStr, ok := s.(string); ok {
				planState.Plan = append(planState.Plan, PlanStep{
					Number:      i + 1,
					Description: stepStr,
					Status:      "pending",
				})
			}
		}
	}

	var result string
	if plan != "" {
		result = fmt.Sprintf("Entered plan mode:\n%s", plan)
	} else if len(planState.Plan) > 0 {
		result = "Entered plan mode with steps:\n"
		for _, s := range planState.Plan {
			result += fmt.Sprintf("  %d. %s\n", s.Number, s.Description)
		}
	} else {
		result = "Entered plan mode. Waiting for plan..."
	}

	if p.onPlanStart != nil {
		p.onPlanStart(plan)
	}

	return loop.LoopResult{
		Success: true,
		Data:    planState,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result}},
		}},
	}
}

// handleExitPlan exits plan mode.
func (p *PlanBucket) handleExitPlan(ctx loop.ExecutionContext) loop.LoopResult {
	completed := planState.IsActive && planState.CurrentStep >= len(planState.Plan)
	
	var summary string
	if completed {
		summary = fmt.Sprintf("Plan completed (%d steps)", len(planState.Plan))
	} else {
		summary = fmt.Sprintf("Plan exited at step %d of %d", planState.CurrentStep, len(planState.Plan))
	}

	planState.IsActive = false
	elapsed := time.Since(planState.StartTime)

	if p.onPlanEnd != nil {
		p.onPlanEnd(completed)
	}

	result := fmt.Sprintf("%s (elapsed: %v)", summary, elapsed)
	
	return loop.LoopResult{
		Success: true,
		Data:    planState.Plan,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result}},
		}},
	}
}

// handleApprovePlan approves and advances the plan.
func (p *PlanBucket) handleApprovePlan(ctx loop.ExecutionContext) loop.LoopResult {
	if !planState.IsActive {
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("not_in_plan_mode", "cannot approve - not in plan mode"),
		}
	}

	stepNum := planState.CurrentStep + 1
	if stepNum <= len(planState.Plan) {
		planState.Plan[stepNum-1].Status = "done"
		planState.CurrentStep = stepNum
	}

	result := fmt.Sprintf("Approved step %d/%d", stepNum, len(planState.Plan))

	if p.onPlanStep != nil {
		if stepNum <= len(planState.Plan) {
			p.onPlanStep(stepNum, len(planState.Plan), planState.Plan[stepNum-1].Description)
		}
	}

	return loop.LoopResult{
		Success: true,
		Data:    result,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result}},
		}},
	}
}

// handleRejectPlan rejects the current plan step.
func (p *PlanBucket) handleRejectPlan(ctx loop.ExecutionContext) loop.LoopResult {
	if !planState.IsActive {
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("not_in_plan_mode", "cannot reject - not in plan mode"),
		}
	}

	reason, _ := ctx.Input["reason"].(string)
	if reason == "" {
		reason = "No reason given"
	}

	stepNum := planState.CurrentStep + 1
	if stepNum <= len(planState.Plan) {
		planState.Plan[stepNum-1].Status = "error"
		planState.Plan[stepNum-1].Result = reason
	}

	result := fmt.Sprintf("Rejected step %d: %s", stepNum, reason)

	return loop.LoopResult{
		Success: false, // Rejection is a failure
		Error:   loop.NewLoopError("step_rejected", result),
		Data:    result,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result, IsError: true}},
		}},
	}
}

// GetPlanState returns current plan state.
func (p *PlanBucket) GetPlanState() *PlanState {
	return planState
}

// IsInPlanMode returns true if currently in plan mode.
func (p *PlanBucket) IsInPlanMode() bool {
	return planState.IsActive
}

// ResetPlan resets the plan state.
func (p *PlanBucket) ResetPlan() {
	planState = &PlanState{}
}

// Ensure LoopPlan implements LoopBase
var _ loop.LoopBase = (*PlanBucket)(nil)
