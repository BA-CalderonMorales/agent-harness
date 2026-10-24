package builtin

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
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
		IsEnabled:               func() bool { return true },
		IsConcurrencySafe:       func(map[string]any) bool { return false },
		IsReadOnly:              func(map[string]any) bool { return true },
		RequiresUserInteraction: func() bool { return true },
		InterruptBehavior:       func() string { return "block" },
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
		return askUser(input, ctx)
	},
	MapResult: func(result any, toolUseID string) types.ToolResultBlock {
		content, _ := result.(string)
		return types.ToolResultBlock{ToolUseID: toolUseID, Content: content}
	},
	UserFacingName: func(map[string]any) string { return "ask" },
})

// askUser routes the question through the host UI when one is wired
// (the TUI), and falls back to a terminal prompt for headless runs.
//
// The fallback reads os.Stdin directly, which is only safe when no
// full-screen program owns the terminal. Under the TUI that is the
// "nasty" case: writes land in the middle of the rendered frame and the
// reader races Bubble Tea for keystrokes. The AskUser callback exists so
// the TUI owns both the rendering and the input.
func askUser(input map[string]any, ctx tools.Context) (tools.ToolResult, error) {
	question := getString(input, "question")
	options := questionOptions(input["options"])

	if ctx.AskUser != nil {
		answer, cancelled, err := ctx.AskUser(ctx.AbortController, question, options)
		if err != nil {
			return tools.ToolResult{}, err
		}
		if cancelled {
			return tools.ToolResult{Data: "The user cancelled the question without answering."}, nil
		}
		return tools.ToolResult{Data: strings.TrimSpace(answer)}, nil
	}

	return askUserOnStdio(question, options, ctx.AbortController)
}

func questionOptions(raw any) []string {
	var options []string
	switch values := raw.(type) {
	case []string:
		options = append(options, values...)
	case []any:
		for _, value := range values {
			if text, ok := value.(string); ok {
				options = append(options, text)
			}
		}
	}
	out := options[:0]
	for _, option := range options {
		if trimmed := strings.TrimSpace(option); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func askUserOnStdio(question string, options []string, abort context.Context) (tools.ToolResult, error) {
	fmt.Printf("\n[Agent asks] %s\n", question)
	for i, option := range options {
		fmt.Printf("  %d) %s\n", i+1, option)
	}

	fmt.Print("Your answer: ")
	if abort == nil {
		abort = context.Background()
	}
	if err := abort.Err(); err != nil {
		return tools.ToolResult{}, err
	}

	answerCh := make(chan struct {
		answer string
		err    error
	}, 1)
	go func() {
		reader := bufio.NewReader(os.Stdin)
		answer, err := reader.ReadString('\n')
		answerCh <- struct {
			answer string
			err    error
		}{answer: answer, err: err}
	}()

	select {
	case result := <-answerCh:
		if result.err != nil {
			return tools.ToolResult{}, result.err
		}
		return tools.ToolResult{Data: strings.TrimSpace(result.answer)}, nil
	case <-abort.Done():
		return tools.ToolResult{}, abort.Err()
	}
}
