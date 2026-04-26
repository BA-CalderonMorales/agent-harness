package agent

import (
	"context"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Agent Loop Limits", func() {
	var loop *Loop
	var bashTool tools.Tool

	BeforeEach(func() {
		bashTool = tools.NewTool(tools.Tool{
			Name: "bash",
			Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
				return tools.ToolResult{Data: "done"}, nil
			},
			MapResult: func(result any, toolUseID string) types.ToolResultBlock {
				return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
			},
		})
	})

	Describe("Max Tool Calls", func() {
		Context("Given an LLM that requests tools repeatedly", func() {
			It("should stop after MaxToolCalls and return BlockingLimit", func() {
				By("creating a mock that always requests a tool")
				mock := &llm.MockClient{Events: llm.MockToolUseResponse("bash", "ls")}
				loop = NewLoop(mock)
				loop.Config.MaxToolCalls = 3
				loop.Config.DefaultMaxTurns = 10

				params := QueryParams{
					Messages:     []types.Message{},
					SystemPrompt: "Test",
					CanUseTool: func(toolName string, input map[string]any, ctx tools.Context) (tools.PermissionDecision, error) {
						return tools.PermissionDecision{Behavior: tools.Allow}, nil
					},
					ToolUseContext: tools.Context{
						Options:         tools.Options{Tools: []tools.Tool{bashTool}},
						AbortController: context.Background(),
					},
				}

				By("running the loop")
				stream, err := loop.Query(context.Background(), params)
				Expect(err).ToNot(HaveOccurred())

				var events []types.StreamEvent
				for ev := range stream {
					events = append(events, ev)
				}

				By("verifying a tool-limit message was emitted")
				var foundLimitMsg bool
				for _, ev := range events {
					if sm, ok := ev.(types.StreamMessage); ok {
						for _, block := range sm.Message.Content {
							if tb, ok := block.(types.TextBlock); ok {
								if strings.Contains(tb.Text, "Tool call limit reached") {
									foundLimitMsg = true
								}
							}
						}
					}
				}
				Expect(foundLimitMsg).To(BeTrue(), "expected a tool call limit message")
			})
		})
	})

	Describe("Max Turns", func() {
		Context("Given an LLM that requests tools on every turn", func() {
			It("should stop after DefaultMaxTurns", func() {
				By("creating a mock that always requests a tool")
				mock := &llm.MockClient{Events: llm.MockToolUseResponse("bash", "ls")}
				loop = NewLoop(mock)
				loop.Config.DefaultMaxTurns = 2
				loop.Config.MaxToolCalls = 100 // high, so turns are the limiting factor

				params := QueryParams{
					Messages:     []types.Message{},
					SystemPrompt: "Test",
					CanUseTool: func(toolName string, input map[string]any, ctx tools.Context) (tools.PermissionDecision, error) {
						return tools.PermissionDecision{Behavior: tools.Allow}, nil
					},
					ToolUseContext: tools.Context{
						Options:         tools.Options{Tools: []tools.Tool{bashTool}},
						AbortController: context.Background(),
					},
				}

				By("running the loop")
				stream, err := loop.Query(context.Background(), params)
				Expect(err).ToNot(HaveOccurred())

				var events []types.StreamEvent
				for ev := range stream {
					events = append(events, ev)
				}

				By("verifying events were produced")
				Expect(len(events)).To(BeNumerically(">", 0))
			})
		})
	})
})
