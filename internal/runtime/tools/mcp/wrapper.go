package mcp

import (
	"fmt"

	svcmcp "github.com/BA-CalderonMorales/agent-harness/internal/runtime/services/mcp"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// Wrap converts an MCP tool definition into a harness tools.Tool.
func Wrap(def svcmcp.WrappedToolDef, mgr *svcmcp.Manager) tools.Tool {
	schema := def.InputSchema
	if schema == nil {
		schema = map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
	}

	return tools.NewTool(tools.Tool{
		Name:        fmt.Sprintf("mcp_%s_%s", def.ServerName, def.Name),
		Description: def.Description,
		InputSchema: func() map[string]any { return schema },
		Capabilities: tools.CapabilityFlags{
			IsEnabled:               func() bool { return true },
			IsConcurrencySafe:       func(map[string]any) bool { return false },
			IsReadOnly:              func(map[string]any) bool { return false },
			IsDestructive:           func(map[string]any) bool { return true },
			InterruptBehavior:       func() string { return "block" },
			RequiresUserInteraction: func() bool { return false },
		},
		Call: func(input map[string]any, ctx tools.Context, canUseTool tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
			result, err := mgr.CallTool(ctx.AbortController, def.ServerName, def.Name, input)
			if err != nil {
				return tools.ToolResult{}, err
			}
			return tools.ToolResult{Data: result}, nil
		},
		MapResult: func(result any, toolUseID string) types.ToolResultBlock {
			content, _ := result.(string)
			if content == "" {
				content = " "
			}
			return types.ToolResultBlock{ToolUseID: toolUseID, Content: content}
		},
		UserFacingName: func(map[string]any) string {
			return fmt.Sprintf("MCP %s/%s", def.ServerName, def.Name)
		},
		GetActivityDescription: func(map[string]any) string {
			return fmt.Sprintf("Calling MCP tool %s/%s", def.ServerName, def.Name)
		},
	})
}
