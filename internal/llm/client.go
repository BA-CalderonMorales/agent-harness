package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// HTTPClient is a provider-agnostic LLM client supporting OpenRouter and Anthropic.
type HTTPClient struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	Provider   string // "openrouter" or "anthropic"
}

// NewHTTPClient creates an LLM client from environment/config.
func NewHTTPClient(provider, apiKey string) *HTTPClient {
	baseURL := "https://openrouter.ai/api/v1"
	switch provider {
	case "openai":
		baseURL = "https://api.openai.com/v1"
	case "anthropic":
		baseURL = "https://api.anthropic.com/v1"
	case "ollama", "local":
		baseURL = "http://localhost:11434/v1"
	}
	return &HTTPClient{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 120 * time.Second},
		Provider:   provider,
	}
}

// Stream implements Client.
func (c *HTTPClient) Stream(ctx context.Context, req Request) (<-chan types.LLMEvent, error) {
	payload, err := c.buildPayload(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	if c.Provider == "openrouter" {
		httpReq.Header.Set("HTTP-Referer", "https://github.com/BA-CalderonMorales/agent-harness")
		httpReq.Header.Set("X-Title", "agent-harness")
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("LLM API error %d: %s", resp.StatusCode, string(body))
	}

	out := make(chan types.LLMEvent, 32)
	go c.readSSE(ctx, resp.Body, out)
	return out, nil
}

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
		"model":    req.Model,
		"messages": messages,
		"stream":   true,
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

	return json.Marshal(payload)
}

func (c *HTTPClient) convertMessage(m types.Message) map[string]any {
	role := string(m.Role)
	if role == "tool" {
		role = "user" // OpenAI compat: tool results go in user messages
	}
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
		case types.ToolUseBlock:
			out = append(out, map[string]any{
				"type":  "tool_use",
				"id":    v.ID,
				"name":  v.Name,
				"input": v.Input,
			})
		case types.ToolResultBlock:
			out = append(out, map[string]any{
				"type":        "tool_result",
				"tool_use_id": v.ToolUseID,
				"content":     v.Content,
				"is_error":    v.IsError,
			})
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

func (c *HTTPClient) readSSE(ctx context.Context, body io.ReadCloser, out chan<- types.LLMEvent) {
	defer close(out)
	defer body.Close()

	reader := bufio.NewReader(body)
	var currentMessageID string
	var currentToolUse *struct {
		id   string
		name string
	}
	var toolInputBuffer strings.Builder

	for {
		select {
		case <-ctx.Done():
			out <- types.LLMError{Error: ctx.Err()}
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				out <- types.LLMError{Error: err}
			}
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			return
		}

		var ev map[string]any
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			continue
		}

		choices, _ := ev["choices"].([]any)
		if len(choices) == 0 {
			continue
		}
		choice, _ := choices[0].(map[string]any)
		delta, _ := choice["delta"].(map[string]any)

		if id, ok := ev["id"].(string); ok && currentMessageID == "" {
			currentMessageID = id
			out <- types.LLMMessageStart{ID: id}
		}

		if content, ok := delta["content"].(string); ok && content != "" {
			out <- types.LLMTextDelta{Delta: content}
		}

		// Tool calls arrive as partial JSON on OpenAI-compatible APIs
		if toolCalls, ok := delta["tool_calls"].([]any); ok && len(toolCalls) > 0 {
			tc, _ := toolCalls[0].(map[string]any)
			if tcID, ok := tc["id"].(string); ok && tcID != "" && (currentToolUse == nil || currentToolUse.id != tcID) {
				if currentToolUse != nil {
					// Emit previous tool use
					toolInputBuffer.Reset()
				}
				fn, _ := tc["function"].(map[string]any)
				name, _ := fn["name"].(string)
				currentToolUse = &struct{ id, name string }{id: tcID, name: name}
			}
			if fn, ok := tc["function"].(map[string]any); ok {
				if arg, ok := fn["arguments"].(string); ok {
					toolInputBuffer.WriteString(arg)
				}
			}
		}

		if finish, ok := choice["finish_reason"].(string); ok && finish != "" {
			if currentToolUse != nil {
				_ = c.parseToolInput(toolInputBuffer.String())
				out <- types.LLMToolUseDelta{ID: currentToolUse.id, Name: currentToolUse.name, Delta: toolInputBuffer.String()}
				currentToolUse = nil
				toolInputBuffer.Reset()
			}
			usage := types.TokenUsage{}
			if u, ok := ev["usage"].(map[string]any); ok {
				// Try to parse usage if present
				usage.InputTokens = intValue(u["prompt_tokens"])
				usage.OutputTokens = intValue(u["completion_tokens"])
			}
			out <- types.LLMMessageStop{StopReason: finish, Usage: usage}
		}
	}
}

func (c *HTTPClient) parseToolInput(raw string) string {
	// For SSE streaming, arguments are already complete JSON strings.
	return raw
}

func intValue(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}
