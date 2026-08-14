package llm

import (
	"encoding/json"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"strings"
)

func (c *HTTPClient) buildPayload(req Request) ([]byte, error) {
	messages := make([]map[string]any, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		messages = append(messages, map[string]any{
			"role":    "system",
			"content": req.SystemPrompt,
		})
	}
	for _, m := range req.Messages {
		messages = append(messages, c.convertMessage(m))
	}

	toolsPayload := make([]map[string]any, 0, len(req.Tools))
	for _, t := range req.Tools {
		schema := t.InputSchema()
		if schema == nil && t.InputJSONSchema != nil {
			schema = t.InputJSONSchema
		}
		toolsPayload = append(toolsPayload, map[string]any{
			"type":     "function",
			"function": map[string]any{"name": t.Name, "description": t.Description, "parameters": schema},
		})
	}

	payload := map[string]any{
		"model":          req.Model,
		"messages":       messages,
		"stream":         true,
		"stream_options": map[string]any{"include_usage": true},
	}
	if req.MaxTokens > 0 {
		payload["max_tokens"] = req.MaxTokens
	}
	if req.Temperature > 0 {
		payload["temperature"] = req.Temperature
	}
	if len(toolsPayload) > 0 {
		payload["tools"] = toolsPayload
	}
	// Anthropic-style thinking via extra_body on OpenRouter
	if req.ThinkingBudget > 0 && c.Provider == "anthropic" {
		payload["thinking"] = map[string]any{"type": "enabled", "budget_tokens": req.ThinkingBudget}
	}
	// NVIDIA's hosted API has no reasoning_effort; it budgets thinking via
	// extra_body (chat_template_kwargs.enable_thinking + reasoning_budget).
	if c.Provider == "nvidia" {
		if req.ReasoningEffort != "" && req.ReasoningEffort != "off" {
			payload["extra_body"] = map[string]any{
				"chat_template_kwargs": map[string]any{"enable_thinking": true},
				"reasoning_budget":     nvidiaReasoningBudget(req.ReasoningEffort),
			}
		}
		return json.Marshal(payload)
	}

	// OpenAI-compatible reasoning effort (OpenRouter passthrough, OpenAI,
	// and local gateways that support it). Ignored when unset or off.
	switch req.ReasoningEffort {
	case "low", "medium", "high":
		payload["reasoning_effort"] = req.ReasoningEffort
	}

	return json.Marshal(payload)
}

// nvidiaReasoningBudget maps the harness effort profiles to NVIDIA's
// reasoning_budget tokens (the free tier caps at 16384).
func nvidiaReasoningBudget(effort string) int {
	switch effort {
	case "low":
		return 1024
	case "high":
		return 16384
	default:
		return 4096
	}
}

func (c *HTTPClient) convertMessage(m types.Message) map[string]any {
	// Tool result messages: OpenAI uses role "tool" with tool_call_id
	if len(m.Content) == 1 {
		if tr, ok := m.Content[0].(types.ToolResultBlock); ok {
			return map[string]any{
				"role":         "tool",
				"tool_call_id": tr.ToolUseID,
				"content":      tr.Content,
			}
		}
	}

	// Assistant messages with tool calls: OpenAI uses top-level tool_calls array
	if m.Role == types.RoleAssistant {
		var toolCalls []map[string]any
		var textParts []string
		for _, block := range m.Content {
			switch v := block.(type) {
			case types.TextBlock:
				textParts = append(textParts, v.Text)
			case types.ToolUseBlock:
				inputJSON, _ := json.Marshal(v.Input)
				toolCalls = append(toolCalls, map[string]any{
					"id":   v.ID,
					"type": "function",
					"function": map[string]any{
						"name":      v.Name,
						"arguments": string(inputJSON),
					},
				})
			}
		}
		msg := map[string]any{"role": "assistant"}
		if len(textParts) > 0 {
			msg["content"] = strings.Join(textParts, "")
		} else {
			msg["content"] = nil
		}
		if len(toolCalls) > 0 {
			msg["tool_calls"] = toolCalls
		}
		return msg
	}

	// Default: user / system messages with standard content blocks
	role := string(m.Role)
	content := c.convertContent(m.Content)
	return map[string]any{"role": role, "content": content}
}

func (c *HTTPClient) convertContent(blocks []types.ContentBlock) any {
	if len(blocks) == 1 {
		if tb, ok := blocks[0].(types.TextBlock); ok {
			return tb.Text
		}
	}
	out := make([]map[string]any, 0, len(blocks))
	for _, b := range blocks {
		switch v := b.(type) {
		case types.TextBlock:
			out = append(out, map[string]any{"type": "text", "text": v.Text})
		case types.ThinkingBlock:
			out = append(out, map[string]any{
				"type":      "thinking",
				"thinking":  v.Thinking,
				"signature": v.Signature,
			})
		}
	}
	return out
}
