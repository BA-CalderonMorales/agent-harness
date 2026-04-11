// Package buckets provides domain-specific LoopBase implementations.
package buckets

import (
	"context"
	"fmt"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/loop"
	"github.com/BA-CalderonMorales/agent-harness/internal/loop/buckets/defaults"
	"github.com/BA-CalderonMorales/agent-harness/internal/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"github.com/google/uuid"
)

// LoopAgent handles sub-agent spawning and delegation.
// This bucket creates isolated agent loops for specific tasks.
type AgentBucket struct {
	llmClient     llm.Client
	toolRegistry  *tools.ToolRegistry
	maxDepth      int
	basePath      string
	orchestratorFactory func(basePath string, client llm.Client) *loop.OrchestrationBucket
}

// NewLoopAgent creates an agent bucket.
func Agent(basePath string, client llm.Client) *AgentBucket {
	return &AgentBucket{
		llmClient:    client,
		toolRegistry: tools.NewRegistry(),
		maxDepth:     defaults.AgentMaxDepthDefault,
		basePath:     basePath,
	}
}

// WithMaxDepth sets the maximum recursion depth.
func (a *AgentBucket) WithMaxDepth(depth int) *AgentBucket {
	a.maxDepth = depth
	return a
}

// WithToolRegistry sets the tool registry for sub-agents.
func (a *AgentBucket) WithToolRegistry(registry *tools.ToolRegistry) *AgentBucket {
	a.toolRegistry = registry
	return a
}

// WithOrchestratorFactory sets a custom factory for creating orchestrators.
func (a *AgentBucket) WithOrchestratorFactory(factory func(basePath string, client llm.Client) *loop.OrchestrationBucket) *AgentBucket {
	a.orchestratorFactory = factory
	return a
}

// Name returns the bucket identifier.
func (a *AgentBucket) Name() string {
	return "agent"
}

// CanHandle determines if this bucket handles the tool.
func (a *AgentBucket) CanHandle(toolName string, input map[string]any) bool {
	return toolName == "agent" || toolName == "sub_agent" || toolName == "delegate"
}

// Capabilities describes what this bucket can do.
func (a *AgentBucket) Capabilities() loop.BucketCapabilities {
	return loop.BucketCapabilities{
		IsConcurrencySafe: false, // Agents run serially
		IsReadOnly:        false,
		IsDestructive:     false, // Sub-agents are isolated
		ToolNames:         []string{"agent", "sub_agent", "delegate"},
		Category:          "agent",
	}
}

// Execute runs the agent delegation.
func (a *AgentBucket) Execute(ctx loop.ExecutionContext) loop.LoopResult {
	// Check recursion depth
	currentDepth := 0
	if d, ok := ctx.Input["_depth"].(float64); ok {
		currentDepth = int(d)
	}
	if currentDepth >= a.maxDepth {
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("depth_exceeded", fmt.Sprintf("agent recursion limit (%d) reached", a.maxDepth)),
			ShouldHalt: true,
		}
	}

	prompt, ok := ctx.Input["prompt"].(string)
	if !ok || prompt == "" {
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("invalid_input", "prompt is required"),
		}
	}

	agentType, _ := ctx.Input["agent_type"].(string)
	if agentType == "" {
		agentType = "default"
	}

	// Create sub-agent context
	subCtx := context.Background()
	if ctx.Context != nil {
		subCtx = ctx.Context
	}

	// Build sub-agent messages
	subMessages := []types.Message{
		{
			UUID:      uuid.New().String(),
			Role:      types.RoleSystem,
			Content:   []types.ContentBlock{types.TextBlock{Text: fmt.Sprintf("You are a %s agent. Focus on the specific task given.", agentType)}},
			Timestamp: time.Now(),
		},
		{
			UUID:      uuid.New().String(),
			Role:      types.RoleUser,
			Content:   []types.ContentBlock{types.TextBlock{Text: prompt}},
			Timestamp: time.Now(),
		},
	}

	// Check if we should use a real orchestrator or mock
	if a.orchestratorFactory != nil && a.llmClient != nil {
		return a.runWithOrchestrator(subCtx, subMessages, ctx.ToolUseID, agentType)
	}

	// Return mock result for pattern demonstration
	result := fmt.Sprintf("[Sub-agent %s completed]\nTask: %s\nResult: (sub-agent execution would run here)", agentType, prompt)
	
	return loop.LoopResult{
		Success: true,
		Data:    result,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{
				ToolUseID: ctx.ToolUseID,
				Content:   result,
			}},
		}},
	}
}

// GetAgentPrompt returns the system prompt for an agent type.
func GetAgentPrompt(agentType string) string {
	if prompt, ok := defaults.AgentTypes[agentType]; ok {
		return prompt
	}
	return defaults.AgentTypes["default"]
}

// runWithOrchestrator executes a real sub-agent using the loop orchestrator.
func (a *AgentBucket) runWithOrchestrator(ctx context.Context, messages []types.Message, toolUseID string, agentType string) loop.LoopResult {
	// Create orchestrator for sub-agent
	var orch *loop.OrchestrationBucket
	if a.orchestratorFactory != nil {
		orch = a.orchestratorFactory(a.basePath, a.llmClient)
	} else {
		// Default: create standard orchestrator
		cfg := loop.DefaultConfig()
		cfg.MaxTurns = 5 // Shorter for sub-agents
		
		// Create buckets for sub-agent
		fs := FileSystem(a.basePath)
		search := Search(a.basePath)
		
		orch = loop.Orchestration(cfg, a.llmClient, fs, search)
	}

	// Set up event channel to capture results
	events := make(chan types.StreamEvent, 32)
	orch.SetEventChannel(events)

	// Build query params
	params := loop.QueryParams{
		Messages:     messages,
		SystemPrompt: GetAgentPrompt(agentType),
		ToolUseContext: tools.Context{
			Options: tools.Options{
				MainLoopModel: "gpt-4o-mini", // Use smaller model for sub-agents
				Tools:         a.toolRegistry.FilterEnabled(),
			},
		},
		MaxTurns: 5,
	}

	// Run the sub-agent
	state, err := orch.Run(ctx, params)
	
	// Collect result from final message
	var result string
	if err != nil {
		result = fmt.Sprintf("Sub-agent error: %v", err)
		return loop.LoopResult{
			Success: false,
			Error:   loop.WrapError("agent_failed", err),
			Data:    result,
			Messages: []types.Message{{
				Role:    types.RoleUser,
				Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: toolUseID, Content: result, IsError: true}},
			}},
		}
	}

	// Extract final assistant message
	if len(state.Messages) > 0 {
		lastMsg := state.Messages[len(state.Messages)-1]
		if lastMsg.Role == types.RoleAssistant {
			for _, block := range lastMsg.Content {
				if text, ok := block.(types.TextBlock); ok {
					result = text.Text
					break
				}
			}
		}
	}

	if result == "" {
		result = "[Sub-agent completed with no text output]"
	}

	return loop.LoopResult{
		Success: true,
		Data:    result,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: toolUseID, Content: result}},
		}},
	}
}

// AgentConfig configures sub-agent behavior.
type AgentConfig struct {
	Model       string
	MaxTurns    int
	SystemPrompt string
	Tools       []tools.Tool
}

// DefaultAgentConfig returns safe defaults for sub-agents.
func DefaultAgentConfig() AgentConfig {
	return AgentConfig{
		Model:        "gpt-4o-mini",
		MaxTurns:     5,
		SystemPrompt: "You are a specialized sub-agent. Focus on the specific task given.",
		Tools:        []tools.Tool{},
	}
}

// Ensure LoopAgent implements LoopBase
var _ loop.LoopBase = (*AgentBucket)(nil)
