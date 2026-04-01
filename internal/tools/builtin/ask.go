package builtin

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// AskUserQuestionTool pauses execution to ask the user a question.
var AskUserQuestionTool = tools.NewTool(tools.Tool{
	Name:        "ask_user_question",
	Description: "Ask the user a clarifying question. Use this when you need more information to proceed.",
	InputSchema: func() map[string]any {
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"question": map[string]any{
					"type":        "string",
					"description": "The question to ask the user",
				},
				"options": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Optional preset answers",
				},
			},
			"required": []string{"question"},
		}
	},
	Capabilities: tools.CapabilityFlags{
		IsEnabled:             func() bool { return true },
		IsConcurrencySafe:     func(map[string]any) bool { return false },
		IsReadOnly:            func(map[string]any) bool { return true },
		RequiresUserInteraction: func() bool { return true },
		InterruptBehavior:     func() string { return "block" },
	},
	ValidateInput: func(input map[string]any, ctx tools.Context) tools.ValidationResult {
		q := getString(input, "question")
		if q == "" {
			return tools.ValidationResult{Valid: false, Message: "question is required"}
		}
		return tools.ValidationResult{Valid: true}
	},
	CheckPermissions: func(input map[string]any, ctx tools.Context) tools.PermissionDecision {
		return tools.PermissionDecision{Behavior: tools.Allow, UpdatedInput: input}
	},
	Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
		question := getString(input, "question")
		fmt.Printf("\n[Agent asks] %s\n", question)

		if opts, ok := input["options"].([]any); ok && len(opts) > 0 {
			fmt.Println("Options:")
			for i, o := range opts {
				fmt.Printf("  %d) %v\n", i+1, o)
			}
		}

		fmt.Print("Your answer: ")
		reader := bufio.NewReader(os.Stdin)
		answer, err := reader.ReadString('\n')
		if err != nil {
			return tools.ToolResult{}, err
		}
		answer = strings.TrimSpace(answer)
		return tools.ToolResult{Data: answer}, nil
	},
	MapResult: func(result any, toolUseID string) types.ToolResultBlock {
		content, _ := result.(string)
		return types.ToolResultBlock{ToolUseID: toolUseID, Content: content}
	},
	UserFacingName: func(map[string]any) string { return "ask" },
})
