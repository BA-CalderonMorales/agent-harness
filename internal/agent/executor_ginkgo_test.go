package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// ---------------------------------------------------------------------------
// Mock tool factories
// ---------------------------------------------------------------------------

func makeImmediateTool(name string, safe bool) tools.Tool {
	return tools.NewTool(tools.Tool{
		Name: name,
		Capabilities: tools.CapabilityFlags{
			IsConcurrencySafe: func(map[string]any) bool { return safe },
			InterruptBehavior: func() string { return "cancel" },
		},
		Call: func(input map[string]any, ctx tools.Context, canUseTool tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
			return tools.ToolResult{Data: name + "-result"}, nil
		},
		MapResult: func(result any, toolUseID string) types.ToolResultBlock {
			return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
		},
	})
}

func makeBlockingTool(name string, safe bool, block <-chan struct{}) tools.Tool {
	return tools.NewTool(tools.Tool{
		Name: name,
		Capabilities: tools.CapabilityFlags{
			IsConcurrencySafe: func(map[string]any) bool { return safe },
			InterruptBehavior: func() string { return "cancel" },
		},
		Call: func(input map[string]any, ctx tools.Context, canUseTool tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
			select {
			case <-block:
			case <-ctx.AbortController.Done():
				return tools.ToolResult{}, ctx.AbortController.Err()
			}
			return tools.ToolResult{Data: name + "-result"}, nil
		},
		MapResult: func(result any, toolUseID string) types.ToolResultBlock {
			return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
		},
	})
}

func makeSignalTool(name string, safe bool, started chan<- struct{}) tools.Tool {
	return tools.NewTool(tools.Tool{
		Name: name,
		Capabilities: tools.CapabilityFlags{
			IsConcurrencySafe: func(map[string]any) bool { return safe },
			InterruptBehavior: func() string { return "cancel" },
		},
		Call: func(input map[string]any, ctx tools.Context, canUseTool tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
			if started != nil {
				close(started)
			}
			return tools.ToolResult{Data: name + "-result"}, nil
		},
		MapResult: func(result any, toolUseID string) types.ToolResultBlock {
			return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
		},
	})
}

func makeProgressTool(name string) tools.Tool {
	return tools.NewTool(tools.Tool{
		Name: name,
		Capabilities: tools.CapabilityFlags{
			IsConcurrencySafe: func(map[string]any) bool { return true },
			InterruptBehavior: func() string { return "cancel" },
		},
		Call: func(input map[string]any, ctx tools.Context, canUseTool tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
			if onProgress != nil {
				onProgress("step-1")
				onProgress("step-2")
			}
			return tools.ToolResult{Data: name + "-result"}, nil
		},
		MapResult: func(result any, toolUseID string) types.ToolResultBlock {
			return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
		},
	})
}

func makeFailingTool(name string, safe bool) tools.Tool {
	return tools.NewTool(tools.Tool{
		Name: name,
		Capabilities: tools.CapabilityFlags{
			IsConcurrencySafe: func(map[string]any) bool { return safe },
			InterruptBehavior: func() string { return "cancel" },
		},
		Call: func(input map[string]any, ctx tools.Context, canUseTool tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
			return tools.ToolResult{}, fmt.Errorf("%s failed", name)
		},
		MapResult: func(result any, toolUseID string) types.ToolResultBlock {
			return types.ToolResultBlock{ToolUseID: toolUseID, Content: fmt.Sprintf("%v", result)}
		},
	})
}

func makeValidationFailingTool(name string) tools.Tool {
	return tools.NewTool(tools.Tool{
		Name: name,
		Capabilities: tools.CapabilityFlags{
			IsConcurrencySafe: func(map[string]any) bool { return true },
			InterruptBehavior: func() string { return "cancel" },
		},
		ValidateInput: func(input map[string]any, ctx tools.Context) tools.ValidationResult {
			return tools.ValidationResult{Valid: false, Message: "bad input"}
		},
		Call: func(input map[string]any, ctx tools.Context, canUseTool tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
			return tools.ToolResult{Data: name + "-result"}, nil
		},
		MapResult: func(result any, toolUseID string) types.ToolResultBlock {
			return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
		},
	})
}

func makePermissionDenyTool(name string) tools.Tool {
	return tools.NewTool(tools.Tool{
		Name: name,
		Capabilities: tools.CapabilityFlags{
			IsConcurrencySafe: func(map[string]any) bool { return true },
			InterruptBehavior: func() string { return "cancel" },
		},
		CheckPermissions: func(input map[string]any, ctx tools.Context) tools.PermissionDecision {
			return tools.PermissionDecision{Behavior: tools.Deny, Message: "not allowed"}
		},
		Call: func(input map[string]any, ctx tools.Context, canUseTool tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
			return tools.ToolResult{Data: name + "-result"}, nil
		},
		MapResult: func(result any, toolUseID string) types.ToolResultBlock {
			return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
		},
	})
}

func firstToolResult(msg types.Message) types.ToolResultBlock {
	Expect(msg.Content).ToNot(BeEmpty())
	tr, ok := msg.Content[0].(types.ToolResultBlock)
	Expect(ok).To(BeTrue())
	return tr
}

// ---------------------------------------------------------------------------
// Specs
// ---------------------------------------------------------------------------

var _ = Describe("StreamingToolExecutor", func() {
	var executor *StreamingToolExecutor
	var toolCtx tools.Context

	BeforeEach(func() {
		toolCtx = tools.Context{AbortController: context.Background()}
	})

	AfterEach(func() {
		if executor != nil {
			executor.Close()
		}
	})

	Describe("Tool Registration and Execution", func() {
		Context("Given a registered safe tool", func() {
			It("should execute and return its result", func() {
				defs := []tools.Tool{makeImmediateTool("read", true)}
				executor = NewStreamingToolExecutor(defs, nil, toolCtx)

				executor.AddTool(types.ToolUseBlock{ID: "t1", Name: "read", Input: map[string]any{}}, types.Message{})

				By("waiting for execution to complete")
				results, err := executor.GetRemainingResults(context.Background())
				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))

				By("verifying the result content")
				tr := firstToolResult(results[0])
				Expect(tr.Content).To(Equal("read-result"))
			})
		})

		Context("Given an unregistered tool", func() {
			It("should immediately complete with an error", func() {
				executor = NewStreamingToolExecutor([]tools.Tool{}, nil, toolCtx)

				executor.AddTool(types.ToolUseBlock{ID: "t1", Name: "missing", Input: map[string]any{}}, types.Message{})

				By("waiting for result")
				results, err := executor.GetRemainingResults(context.Background())
				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))

				By("verifying the error message")
				tr := firstToolResult(results[0])
				Expect(tr.IsError).To(BeTrue())
				Expect(tr.Content).To(ContainSubstring("No such tool available"))
			})
		})

		Context("Given multiple tools", func() {
			It("should return results in enqueue order", func() {
				defs := []tools.Tool{
					makeImmediateTool("read", true),
					makeImmediateTool("write", true),
				}
				executor = NewStreamingToolExecutor(defs, nil, toolCtx)

				executor.AddTool(types.ToolUseBlock{ID: "t1", Name: "read", Input: map[string]any{}}, types.Message{})
				executor.AddTool(types.ToolUseBlock{ID: "t2", Name: "write", Input: map[string]any{}}, types.Message{})

				results, err := executor.GetRemainingResults(context.Background())
				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(2))

				tr1 := firstToolResult(results[0])
				tr2 := firstToolResult(results[1])
				Expect(tr1.Content).To(Equal("read-result"))
				Expect(tr2.Content).To(Equal("write-result"))
			})
		})
	})

	Describe("Validation and Permissions", func() {
		Context("Given a tool with failing validation", func() {
			It("should error without calling the tool", func() {
				defs := []tools.Tool{makeValidationFailingTool("bad")}
				executor = NewStreamingToolExecutor(defs, nil, toolCtx)

				executor.AddTool(types.ToolUseBlock{ID: "t1", Name: "bad", Input: map[string]any{}}, types.Message{})

				results, err := executor.GetRemainingResults(context.Background())
				Expect(err).ToNot(HaveOccurred())

				tr := firstToolResult(results[0])
				Expect(tr.IsError).To(BeTrue())
				Expect(tr.Content).To(ContainSubstring("validation failed"))
			})
		})

		Context("Given a tool with permission denied", func() {
			It("should error without calling the tool", func() {
				defs := []tools.Tool{makePermissionDenyTool("secret")}
				executor = NewStreamingToolExecutor(defs, nil, toolCtx)

				executor.AddTool(types.ToolUseBlock{ID: "t1", Name: "secret", Input: map[string]any{}}, types.Message{})

				results, err := executor.GetRemainingResults(context.Background())
				Expect(err).ToNot(HaveOccurred())

				tr := firstToolResult(results[0])
				Expect(tr.IsError).To(BeTrue())
				Expect(tr.Content).To(ContainSubstring("permission denied"))
			})
		})
	})

	Describe("Event Streaming", func() {
		Context("Given a tool that reports progress", func() {
			It("should stream progress events before completion", func() {
				defs := []tools.Tool{makeProgressTool("read")}
				executor = NewStreamingToolExecutor(defs, nil, toolCtx)

				executor.AddTool(types.ToolUseBlock{ID: "t1", Name: "read", Input: map[string]any{}}, types.Message{})
				_, _ = executor.GetRemainingResults(context.Background())

				By("collecting all buffered events after close")
				executor.Close()
				var events []types.StreamEvent
				for ev := range executor.Events() {
					events = append(events, ev)
				}

				By("checking progress events were received")
				var progressCount int
				for _, ev := range events {
					if _, ok := ev.(types.ProgressMessage); ok {
						progressCount++
					}
				}
				Expect(progressCount).To(Equal(2))

				By("checking completion event was received")
				var completionCount int
				for _, ev := range events {
					if _, ok := ev.(types.StreamMessage); ok {
						completionCount++
					}
				}
				Expect(completionCount).To(Equal(1))
			})
		})
	})

	Describe("Error Handling", func() {
		Context("Given a tool that returns an error", func() {
			It("should surface the error in the result", func() {
				defs := []tools.Tool{makeFailingTool("crash", true)}
				executor = NewStreamingToolExecutor(defs, nil, toolCtx)

				executor.AddTool(types.ToolUseBlock{ID: "t1", Name: "crash", Input: map[string]any{}}, types.Message{})

				results, err := executor.GetRemainingResults(context.Background())
				Expect(err).ToNot(HaveOccurred())

				tr := firstToolResult(results[0])
				Expect(tr.IsError).To(BeTrue())
				Expect(tr.Content).To(ContainSubstring("crash failed"))
			})
		})
	})

	Describe("Discard Behavior", func() {
		Context("Given a slow tool and discard is called", func() {
			It("should override the result with a discard message", func() {
				block := make(chan struct{})
				defs := []tools.Tool{makeBlockingTool("slow", true, block)}
				executor = NewStreamingToolExecutor(defs, nil, toolCtx)

				executor.AddTool(types.ToolUseBlock{ID: "t1", Name: "slow", Input: map[string]any{}}, types.Message{})

				By("waiting for the tool to start executing")
				time.Sleep(50 * time.Millisecond)

				executor.Discard()

				By("unblocking the tool so the executor can finish")
				close(block)

				results, err := executor.GetRemainingResults(context.Background())
				Expect(err).ToNot(HaveOccurred())

				tr := firstToolResult(results[0])
				Expect(tr.Content).To(ContainSubstring("discarded"))
			})
		})
	})

	Describe("Concurrency Safety", func() {
		Context("Given a blocking unsafe tool followed by a safe tool", func() {
			It("should execute the safe tool only after unsafe completes", func() {
				unsafeBlock := make(chan struct{})
				readStarted := make(chan struct{})

				defs := []tools.Tool{
					makeBlockingTool("write", false, unsafeBlock),
					makeSignalTool("read", true, readStarted),
				}
				executor = NewStreamingToolExecutor(defs, nil, toolCtx)

				executor.AddTool(types.ToolUseBlock{ID: "t1", Name: "write", Input: map[string]any{}}, types.Message{})
				executor.AddTool(types.ToolUseBlock{ID: "t2", Name: "read", Input: map[string]any{}}, types.Message{})

				By("waiting for queue to settle")
				time.Sleep(50 * time.Millisecond)

				By("verifying read has not started while write is executing")
				select {
				case <-readStarted:
					Fail("read started before write completed")
				default:
				}

				By("unblocking the unsafe tool")
				close(unsafeBlock)

				By("waiting for read to start after write completes")
				select {
				case <-readStarted:
				case <-time.After(2 * time.Second):
					Fail("read did not start after write completed")
				}

				results, err := executor.GetRemainingResults(context.Background())
				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(2))

				tr1 := firstToolResult(results[0])
				tr2 := firstToolResult(results[1])
				Expect(tr1.Content).To(Equal("write-result"))
				Expect(tr2.Content).To(Equal("read-result"))
			})
		})
	})
})
