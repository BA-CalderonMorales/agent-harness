package llm

import (
	"encoding/json"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestLLMClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LLM Client Suite")
}

var _ = Describe("HTTPClient Payload Format", func() {
	var client *HTTPClient

	BeforeEach(func() {
		client = NewHTTPClient("openrouter", "test-key")
	})

	Describe("buildPayload", func() {
		Context("Given a conversation with tool calls and results", func() {
			It("should produce OpenAI-compatible message format", func() {
				By("building a request with user message, assistant tool call, and tool result")
				req := Request{
					Messages: []types.Message{
						{
							Role:    types.RoleUser,
							Content: []types.ContentBlock{types.TextBlock{Text: "List files"}},
						},
						{
							Role: types.RoleAssistant,
							Content: []types.ContentBlock{
								types.TextBlock{Text: "Let me check."},
								types.ToolUseBlock{ID: "call_abc", Name: "ls", Input: map[string]any{"path": "/home"}},
							},
						},
						{
							Role:    types.RoleUser,
							Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: "call_abc", Content: "alpha.txt\nbeta.go"}},
						},
					},
					SystemPrompt: "You are a helper.",
					Tools: []tools.Tool{
						{
							Name:        "ls",
							Description: "List directory",
							InputSchema: func() map[string]any { return map[string]any{"type": "object"} },
						},
					},
					Model: "test-model",
				}

				payload, err := client.buildPayload(req)
				Expect(err).ToNot(HaveOccurred())

				var result map[string]any
				Expect(json.Unmarshal(payload, &result)).To(Succeed())

				By("verifying messages are in OpenAI format")
				messages, ok := result["messages"].([]any)
				Expect(ok).To(BeTrue(), "messages should be an array")
				Expect(messages).To(HaveLen(4)) // system + user + assistant + tool

				// System message
				sysMsg, _ := messages[0].(map[string]any)
				Expect(sysMsg["role"]).To(Equal("system"))

				// User message
				userMsg, _ := messages[1].(map[string]any)
				Expect(userMsg["role"]).To(Equal("user"))
				Expect(userMsg["content"]).To(Equal("List files"))

				// Assistant message with tool call
				assistantMsg, _ := messages[2].(map[string]any)
				Expect(assistantMsg["role"]).To(Equal("assistant"))
				Expect(assistantMsg["content"]).To(Equal("Let me check."))

				toolCalls, ok := assistantMsg["tool_calls"].([]any)
				Expect(ok).To(BeTrue(), "assistant message should have tool_calls")
				Expect(toolCalls).To(HaveLen(1))

				tc, _ := toolCalls[0].(map[string]any)
				Expect(tc["id"]).To(Equal("call_abc"))
				Expect(tc["type"]).To(Equal("function"))

				fn, _ := tc["function"].(map[string]any)
				Expect(fn["name"]).To(Equal("ls"))
				Expect(fn["arguments"]).To(Equal(`{"path":"/home"}`))

				// Tool result message
				toolMsg, _ := messages[3].(map[string]any)
				Expect(toolMsg["role"]).To(Equal("tool"))
				Expect(toolMsg["tool_call_id"]).To(Equal("call_abc"))
				Expect(toolMsg["content"]).To(Equal("alpha.txt\nbeta.go"))
			})
		})

		Context("Given an assistant message with only tool calls and no text", func() {
			It("should set content to null and include tool_calls", func() {
				req := Request{
					Messages: []types.Message{
						{
							Role:    types.RoleUser,
							Content: []types.ContentBlock{types.TextBlock{Text: "hi"}},
						},
						{
							Role:    types.RoleAssistant,
							Content: []types.ContentBlock{types.ToolUseBlock{ID: "call_1", Name: "bash", Input: map[string]any{"command": "ls"}}},
						},
					},
					Model: "test-model",
				}

				payload, err := client.buildPayload(req)
				Expect(err).ToNot(HaveOccurred())

				var result map[string]any
				Expect(json.Unmarshal(payload, &result)).To(Succeed())

				messages := result["messages"].([]any)
				assistantMsg := messages[1].(map[string]any)
				Expect(assistantMsg["role"]).To(Equal("assistant"))
				Expect(assistantMsg["content"]).To(BeNil())
				Expect(assistantMsg["tool_calls"]).ToNot(BeNil())
			})
		})

		Context("Given multiple tool results in one turn", func() {
			It("should produce separate tool messages for each result", func() {
				req := Request{
					Messages: []types.Message{
						{
							Role:    types.RoleAssistant,
							Content: []types.ContentBlock{types.ToolUseBlock{ID: "call_a", Name: "read", Input: map[string]any{}}},
						},
						{
							Role:    types.RoleUser,
							Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: "call_a", Content: "file-a"}},
						},
						{
							Role:    types.RoleAssistant,
							Content: []types.ContentBlock{types.ToolUseBlock{ID: "call_b", Name: "read", Input: map[string]any{}}},
						},
						{
							Role:    types.RoleUser,
							Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: "call_b", Content: "file-b"}},
						},
					},
					Model: "test-model",
				}

				payload, err := client.buildPayload(req)
				Expect(err).ToNot(HaveOccurred())

				var result map[string]any
				Expect(json.Unmarshal(payload, &result)).To(Succeed())

				messages := result["messages"].([]any)
				Expect(messages).To(HaveLen(4))

				msgA := messages[1].(map[string]any)
				Expect(msgA["role"]).To(Equal("tool"))
				Expect(msgA["tool_call_id"]).To(Equal("call_a"))

				msgB := messages[3].(map[string]any)
				Expect(msgB["role"]).To(Equal("tool"))
				Expect(msgB["tool_call_id"]).To(Equal("call_b"))
			})
		})

		Context("Given an assistant message with multiple tool calls", func() {
			It("should include all tool_calls in the assistant message", func() {
				req := Request{
					Messages: []types.Message{
						{
							Role: types.RoleAssistant,
							Content: []types.ContentBlock{
								types.ToolUseBlock{ID: "call_1", Name: "ls", Input: map[string]any{"path": "/a"}},
								types.ToolUseBlock{ID: "call_2", Name: "ls", Input: map[string]any{"path": "/b"}},
							},
						},
					},
					Model: "test-model",
				}

				payload, err := client.buildPayload(req)
				Expect(err).ToNot(HaveOccurred())

				var result map[string]any
				Expect(json.Unmarshal(payload, &result)).To(Succeed())

				messages := result["messages"].([]any)
				assistantMsg := messages[0].(map[string]any)
				toolCalls := assistantMsg["tool_calls"].([]any)
				Expect(toolCalls).To(HaveLen(2))

				first := toolCalls[0].(map[string]any)
				Expect(first["id"]).To(Equal("call_1"))
				second := toolCalls[1].(map[string]any)
				Expect(second["id"]).To(Equal("call_2"))
			})
		})

		Context("Given a plain user message with no tools", func() {
			It("should keep content as a simple string", func() {
				req := Request{
					Messages: []types.Message{
						{
							Role:    types.RoleUser,
							Content: []types.ContentBlock{types.TextBlock{Text: "hello"}},
						},
					},
					Model: "test-model",
				}

				payload, err := client.buildPayload(req)
				Expect(err).ToNot(HaveOccurred())

				var result map[string]any
				Expect(json.Unmarshal(payload, &result)).To(Succeed())

				messages := result["messages"].([]any)
				userMsg := messages[0].(map[string]any)
				Expect(userMsg["role"]).To(Equal("user"))
				Expect(userMsg["content"]).To(Equal("hello"))
			})
		})
	})
})
