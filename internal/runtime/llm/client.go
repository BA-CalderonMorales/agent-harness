package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
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
	return NewHTTPClientWithBaseURL(provider, apiKey, "")
}

// NewHTTPClientWithBaseURL creates an LLM client with an optional endpoint override.
func NewHTTPClientWithBaseURL(provider, apiKey, baseURL string) *HTTPClient {
	if baseURL == "" {
		baseURL = defaultBaseURL(provider)
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &HTTPClient{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 120 * time.Second},
		Provider:   provider,
	}
}

func defaultBaseURL(provider string) string {
	baseURL := "https://openrouter.ai/api/v1"
	switch provider {
	case "openai":
		baseURL = "https://api.openai.com/v1"
	case "anthropic":
		baseURL = "https://api.anthropic.com/v1"
	case "ollama":
		baseURL = "http://localhost:11434/v1"
	case "local":
		baseURL = "http://127.0.0.1:8080/v1"
	}
	return baseURL
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
	// Ollama doesn't require an API key
	if c.Provider != "ollama" && c.Provider != "local" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
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
	// OpenAI-compatible reasoning effort (OpenRouter passthrough, OpenAI,
	// and local gateways that support it). Ignored when unset or off.
	switch req.ReasoningEffort {
	case "low", "medium", "high":
		payload["reasoning_effort"] = req.ReasoningEffort
	}

	return json.Marshal(payload)
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

func (c *HTTPClient) readSSE(ctx context.Context, body io.ReadCloser, out chan<- types.LLMEvent) {
	defer close(out)
	defer body.Close()

	readFinished := make(chan struct{})
	defer close(readFinished)
	go func() {
		select {
		case <-ctx.Done():
			_ = body.Close()
		case <-readFinished:
		}
	}()

	type toolCallDelta struct {
		Index    *int   `json:"index"`
		ID       string `json:"id"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}
	type streamChoice struct {
		Delta struct {
			Content   string          `json:"content"`
			ToolCalls []toolCallDelta `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	}
	type streamUsage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		PromptDetails    struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	}
	type streamChunk struct {
		ID      string         `json:"id"`
		Model   string         `json:"model"`
		Choices []streamChoice `json:"choices"`
		Usage   *streamUsage   `json:"usage"`
	}
	type accumulatedToolCall struct {
		id        string
		name      strings.Builder
		arguments strings.Builder
	}

	const (
		maxStreamedToolCalls     = 128
		maxStreamedToolNameBytes = 512
		maxStreamedArguments     = 1 << 20
	)

	reader := bufio.NewReader(body)
	var currentMessageID string
	var model string
	var finishReason string
	var usage types.TokenUsage
	toolCalls := make(map[int]*accumulatedToolCall)

	emit := func(event types.LLMEvent) bool {
		select {
		case out <- event:
			return true
		case <-ctx.Done():
			return false
		}
	}
	emitContextError := func() {
		select {
		case out <- types.LLMError{Error: ctx.Err()}:
		default:
		}
	}
	finalize := func() error {
		if currentMessageID == "" && finishReason == "" && len(toolCalls) == 0 {
			return nil
		}
		if finishReason == "" {
			return fmt.Errorf("SSE stream ended without a finish reason")
		}

		indices := make([]int, 0, len(toolCalls))
		for index := range toolCalls {
			indices = append(indices, index)
		}
		sort.Ints(indices)

		completed := make([]types.LLMToolUseDelta, 0, len(indices))
		for _, index := range indices {
			call := toolCalls[index]
			if call.id == "" {
				return fmt.Errorf("tool call index %d is missing a stable ID", index)
			}
			name := call.name.String()
			if name == "" {
				return fmt.Errorf("tool call index %d (%s) is missing a function name", index, call.id)
			}

			rawArguments := call.arguments.String()
			if rawArguments == "" {
				rawArguments = "{}"
			}
			var object map[string]any
			if err := json.Unmarshal([]byte(rawArguments), &object); err != nil {
				return fmt.Errorf("tool call index %d (%s) has invalid arguments JSON: %w", index, call.id, err)
			}
			if object == nil {
				return fmt.Errorf("tool call index %d (%s) has invalid arguments JSON: expected an object", index, call.id)
			}

			completed = append(completed, types.LLMToolUseDelta{
				ID:    call.id,
				Name:  name,
				Delta: rawArguments,
			})
		}

		for _, event := range completed {
			if !emit(event) {
				return ctx.Err()
			}
		}
		if !emit(types.LLMMessageStop{
			StopReason: finishReason,
			Model:      model,
			Usage:      usage,
		}) {
			return ctx.Err()
		}
		return nil
	}
	fail := func(err error) {
		if err == nil {
			return
		}
		if ctx.Err() != nil {
			emitContextError()
			return
		}
		_ = emit(types.LLMError{Error: err})
	}

	for {
		if ctx.Err() != nil {
			emitContextError()
			return
		}

		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "data:") {
				data := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
				if data == "[DONE]" {
					if finalErr := finalize(); finalErr != nil {
						fail(finalErr)
					}
					return
				}

				var chunk streamChunk
				if unmarshalErr := json.Unmarshal([]byte(data), &chunk); unmarshalErr != nil {
					fail(fmt.Errorf("invalid SSE JSON: %w", unmarshalErr))
					return
				}

				if chunk.ID != "" {
					if currentMessageID == "" {
						currentMessageID = chunk.ID
						if !emit(types.LLMMessageStart{ID: chunk.ID}) {
							emitContextError()
							return
						}
					} else if chunk.ID != currentMessageID {
						fail(fmt.Errorf("SSE message ID changed from %q to %q", currentMessageID, chunk.ID))
						return
					}
				}
				if chunk.Model != "" {
					model = chunk.Model
				}
				if chunk.Usage != nil {
					usage.InputTokens = chunk.Usage.PromptTokens
					usage.OutputTokens = chunk.Usage.CompletionTokens
					usage.CacheReadInputTokens = chunk.Usage.PromptDetails.CachedTokens
				}

				if len(chunk.Choices) > 0 {
					choice := chunk.Choices[0]
					if choice.Delta.Content != "" {
						if !emit(types.LLMTextDelta{Delta: choice.Delta.Content}) {
							emitContextError()
							return
						}
					}

					for _, delta := range choice.Delta.ToolCalls {
						if delta.Index == nil || *delta.Index < 0 {
							fail(fmt.Errorf("tool call delta is missing a valid index"))
							return
						}
						index := *delta.Index
						call, ok := toolCalls[index]
						if !ok {
							if len(toolCalls) >= maxStreamedToolCalls {
								fail(fmt.Errorf("too many streamed tool calls: limit is %d", maxStreamedToolCalls))
								return
							}
							call = &accumulatedToolCall{}
							toolCalls[index] = call
						}

						if delta.ID != "" {
							if call.id != "" && call.id != delta.ID {
								fail(fmt.Errorf("tool call index %d changed ID from %q to %q", index, call.id, delta.ID))
								return
							}
							call.id = delta.ID
						}
						if delta.Function.Name != "" && delta.Function.Name != call.name.String() {
							if call.name.Len()+len(delta.Function.Name) > maxStreamedToolNameBytes {
								fail(fmt.Errorf("tool call index %d function name exceeds %d bytes", index, maxStreamedToolNameBytes))
								return
							}
							call.name.WriteString(delta.Function.Name)
						}
						if delta.Function.Arguments != "" {
							if call.arguments.Len()+len(delta.Function.Arguments) > maxStreamedArguments {
								fail(fmt.Errorf("tool call index %d arguments exceed %d bytes", index, maxStreamedArguments))
								return
							}
							call.arguments.WriteString(delta.Function.Arguments)
						}
					}

					if choice.FinishReason != nil && *choice.FinishReason != "" {
						if finishReason != "" && finishReason != *choice.FinishReason {
							fail(fmt.Errorf("SSE finish reason changed from %q to %q", finishReason, *choice.FinishReason))
							return
						}
						finishReason = *choice.FinishReason
					}
				}
			}
		}

		if err != nil {
			if ctx.Err() != nil {
				emitContextError()
				return
			}
			if err == io.EOF {
				if finalErr := finalize(); finalErr != nil {
					fail(finalErr)
				}
				return
			}
			fail(fmt.Errorf("SSE read failed: %w", err))
			return
		}
	}
}

// modelCache caches fetched model lists per base URL.
type modelCache struct {
	models    []string
	fetchedAt time.Time
}

var (
	modelCacheMu  sync.Mutex
	modelCaches   = make(map[string]*modelCache)
	modelCacheTTL = 5 * time.Minute
)

// ListModels fetches available models from the provider API.
// Results are cached for 5 minutes.
func (c *HTTPClient) ListModels() ([]string, error) {
	if c.Provider == "ollama" || c.Provider == "local" {
		return nil, fmt.Errorf("dynamic model listing not supported for %s", c.Provider)
	}

	modelCacheMu.Lock()
	cache, ok := modelCaches[c.BaseURL]
	if ok && time.Since(cache.fetchedAt) < modelCacheTTL {
		modelCacheMu.Unlock()
		return cache.models, nil
	}
	modelCacheMu.Unlock()

	req, err := http.NewRequest("GET", c.BaseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	if c.Provider != "ollama" && c.Provider != "local" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	if c.Provider == "openrouter" {
		req.Header.Set("HTTP-Referer", "https://github.com/BA-CalderonMorales/agent-harness")
		req.Header.Set("X-Title", "agent-harness")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	models := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		if m.ID != "" {
			models = append(models, m.ID)
		}
	}

	modelCacheMu.Lock()
	modelCaches[c.BaseURL] = &modelCache{models: models, fetchedAt: time.Now()}
	modelCacheMu.Unlock()

	return models, nil
}
