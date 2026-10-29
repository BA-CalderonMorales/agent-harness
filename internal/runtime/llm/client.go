package llm

import (
	"bytes"
	"context"
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"io"
	"net/http"
	"strconv"
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
	case "lemonade":
		baseURL = config.DefaultLemonadeEndpoint
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

// retryableStatus reports the transient failures a hosted endpoint can
// answer before any token exists. A 429 is the signature of a shared free
// pool; 502/503/504 are upstream gateway failures. A 500 is deliberately
// not retried: the request may already have been processed and billed.
func retryableStatus(code int) bool {
	switch code {
	case http.StatusRequestTimeout, http.StatusTooManyRequests,
		http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// maxRetryAfter caps a provider-supplied Retry-After. Waiting longer than
// this turns a fast, actionable rate-limit error into a silent stall; the
// user can retry or switch models instead.
const maxRetryAfter = 3 * time.Second

// retryDelay returns the wait before attempt+1 and whether a retry fits
// inside maxRetryAfter. A Retry-After header outranks exponential backoff
// when it is short enough to honor.
func retryDelay(attempt int, retryAfter string) (time.Duration, bool) {
	if retryAfter != "" {
		if seconds, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil {
			d := time.Duration(seconds) * time.Second
			if d > maxRetryAfter {
				return 0, false
			}
			return d, true
		}
		if when, err := http.ParseTime(retryAfter); err == nil {
			d := time.Until(when)
			if d <= 0 {
				return 0, true
			}
			if d > maxRetryAfter {
				return 0, false
			}
			return d, true
		}
	}
	base := 400 * time.Millisecond
	for i := 1; i < attempt; i++ {
		base *= 3
	}
	return base, true
}

// waitRetry sleeps the backoff, returning false when the turn was
// cancelled instead of making the caller wait out a dead request.
func waitRetry(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// doStreamRequest sends the chat request, retrying only transient
// failures for hosted providers. Local servers are never retried: a dead
// local endpoint should fail immediately and point at the server.
func (c *HTTPClient) doStreamRequest(ctx context.Context, payload []byte) (*http.Response, error) {
	const attempts = 3
	hosted := !config.IsLocalProvider(c.Provider)

	for attempt := 1; attempt <= attempts; attempt++ {
		httpReq, err := c.newStreamRequest(ctx, payload)
		if err != nil {
			return nil, err
		}

		resp, err := c.HTTPClient.Do(httpReq)
		if err != nil {
			if !hosted || attempt == attempts || ctx.Err() != nil {
				return nil, err
			}
			delay, ok := retryDelay(attempt, "")
			if !ok || !waitRetry(ctx, delay) {
				return nil, err
			}
			continue
		}

		if !hosted || !retryableStatus(resp.StatusCode) || attempt == attempts {
			return resp, nil
		}
		// A 429 without a retry window is usually a free-model per-minute
		// or per-day cap, not a momentary spike: re-sending within a
		// second cannot clear it and only burns another request against
		// the same limit. Surface the provider's own detail instead.
		if resp.StatusCode == http.StatusTooManyRequests && resp.Header.Get("Retry-After") == "" {
			return resp, nil
		}

		// Keep the error body: if Retry-After exceeds the cap this is
		// the response the caller must surface. Replaying the request
		// just to reread it would bill a second attempt.
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		_ = resp.Body.Close()

		delay, ok := retryDelay(attempt, resp.Header.Get("Retry-After"))
		if !ok || !waitRetry(ctx, delay) {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			resp.Body = io.NopCloser(bytes.NewReader(body))
			resp.ContentLength = int64(len(body))
			return resp, nil
		}
	}

	return nil, fmt.Errorf("LLM request failed after %d attempts", attempts)
}

func (c *HTTPClient) newStreamRequest(ctx context.Context, payload []byte) (*http.Request, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// Local runtimes don't require an API key
	if !config.IsLocalProvider(c.Provider) {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	if c.Provider == "openrouter" {
		httpReq.Header.Set("HTTP-Referer", "https://github.com/BA-CalderonMorales/agent-harness")
		httpReq.Header.Set("X-Title", "agent-harness")
	}
	return httpReq, nil
}

// Stream implements Client.
func (c *HTTPClient) Stream(ctx context.Context, req Request) (<-chan types.LLMEvent, error) {
	payload, err := c.buildPayload(req)
	if err != nil {
		return nil, err
	}

	resp, err := c.doStreamRequest(ctx, payload)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		// Provider error bodies can echo the request back (OpenAI and
		// OpenRouter include the key prefix in 401 messages); the error
		// surfaces in the chat pane and session files, so the key must
		// never ride along.
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		_ = resp.Body.Close()
		// Keep the provider's sentence AND its metadata: the same
		// "Rate limit exceeded" covers a free-tier daily cap, a busy
		// upstream provider, and a budget rejection. The X-RateLimit-*
		// headers name the cap and its reset window.
		apiErr := &APIError{
			StatusCode: resp.StatusCode,
			Message:    apiErrorMessage(body),
			Metadata:   apiErrorMetadata(body),
			Limit:      resp.Header.Get("X-RateLimit-Limit"),
			Remaining:  resp.Header.Get("X-RateLimit-Remaining"),
			Reset:      resp.Header.Get("X-RateLimit-Reset"),
			RetryAfter: resp.Header.Get("Retry-After"),
		}
		return nil, fmt.Errorf("%s", sanitizeError(apiErr, c.APIKey))
	}

	out := make(chan types.LLMEvent, 32)
	go c.readSSE(ctx, resp.Body, out)
	return out, nil
}
