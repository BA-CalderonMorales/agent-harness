package llm

import (
	"bytes"
	"context"
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"io"
	"net/http"
	"strings"
	"time"
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

// defaultHTTPTimeout guards hosted API calls. Local providers must pass a
// longer window via NewHTTPClientWithBaseURLTimeout: CPU prompt eval can
// take minutes before the first token.
const defaultHTTPTimeout = 120 * time.Second

// NewHTTPClientWithBaseURL creates an LLM client with an optional endpoint override.
func NewHTTPClientWithBaseURL(provider, apiKey, baseURL string) *HTTPClient {
	return NewHTTPClientWithBaseURLTimeout(provider, apiKey, baseURL, defaultHTTPTimeout)
}

// NewHTTPClientWithBaseURLTimeout creates an LLM client with an optional
// endpoint override and an explicit HTTP client timeout.
func NewHTTPClientWithBaseURLTimeout(provider, apiKey, baseURL string, timeout time.Duration) *HTTPClient {
	if baseURL == "" {
		baseURL = defaultBaseURL(provider)
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &HTTPClient{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: timeout},
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
	case "flm":
		baseURL = "http://127.0.0.1:52625/v1"
	case "local":
		baseURL = "http://127.0.0.1:8080/v1"
	case "fireworks":
		baseURL = "https://api.fireworks.ai/inference/v1"
	case "nvidia":
		baseURL = "https://integrate.api.nvidia.com/v1"
	case "omniroute":
		baseURL = "https://api.cheaperinference.com/v1"
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
	// Local runtimes don't require an API key
	if c.Provider != "ollama" && c.Provider != "local" && c.Provider != "flm" {
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
		// Provider error bodies can echo the request back (OpenAI and
		// OpenRouter include the key prefix in 401 messages); the error
		// surfaces in the chat pane and session files, so the key must
		// never ride along.
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		resp.Body.Close()
		return nil, fmt.Errorf("LLM API error %d: %s", resp.StatusCode, sanitizeError(fmt.Errorf("%s", string(body)), c.APIKey))
	}

	out := make(chan types.LLMEvent, 32)
	go c.readSSE(ctx, resp.Body, out)
	return out, nil
}
