package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ProviderReadiness represents the readiness state of a provider.
type ProviderReadiness int

const (
	ProviderChecking ProviderReadiness = iota
	ProviderReady
	ProviderWarning
	ProviderUnavailable
	ProviderMisconfigured
)

func (r ProviderReadiness) String() string {
	switch r {
	case ProviderChecking:
		return "checking"
	case ProviderReady:
		return "ready"
	case ProviderWarning:
		return "warning"
	case ProviderUnavailable:
		return "unavailable"
	case ProviderMisconfigured:
		return "misconfigured"
	default:
		return "unknown"
	}
}

// ProviderProber checks if a provider is ready without making paid API calls.
type ProviderProber interface {
	// Probe checks provider readiness with a bounded timeout.
	// It returns the readiness state and an optional message.
	Probe(ctx context.Context) (ProviderReadiness, string)
}

// HTTPProber implements ProviderProber for HTTP-based providers.
type HTTPProber struct {
	BaseURL  string
	APIKey   string
	Provider string
	Timeout  time.Duration
}

// NewHTTPProber creates a prober for the given provider configuration.
func NewHTTPProber(provider, apiKey, baseURL string) *HTTPProber {
	if baseURL == "" {
		baseURL = defaultBaseURL(provider)
	}
	return &HTTPProber{
		BaseURL:  strings.TrimRight(baseURL, "/"),
		APIKey:   apiKey,
		Provider: provider,
		Timeout:  1500 * time.Millisecond,
	}
}

// Probe checks if the provider is accessible and properly configured.
func (p *HTTPProber) Probe(ctx context.Context) (ProviderReadiness, string) {
	ctx, cancel := context.WithTimeout(ctx, p.Timeout)
	defer cancel()

	switch p.Provider {
	case "ollama":
		return p.probeOllama(ctx)
	case "local":
		return p.probeLocal(ctx)
	default:
		return p.probeOpenAICompatible(ctx)
	}
}

func (p *HTTPProber) probeOpenAICompatible(ctx context.Context) (ProviderReadiness, string) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.BaseURL+"/models", nil)
	if err != nil {
		return ProviderMisconfigured, fmt.Sprintf("invalid request: %v", sanitizeError(err, p.APIKey))
	}

	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	if p.Provider == "openrouter" {
		req.Header.Set("HTTP-Referer", "https://github.com/BA-CalderonMorales/agent-harness")
		req.Header.Set("X-Title", "agent-harness")
	}

	client := &http.Client{Timeout: p.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return ProviderUnavailable, "provider timeout"
		}
		return ProviderUnavailable, sanitizeError(err, p.APIKey)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		var models struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &models); err != nil {
			return ProviderWarning, "invalid response format"
		}
		if len(models.Data) == 0 {
			return ProviderWarning, "no models available"
		}
		return ProviderReady, fmt.Sprintf("%d models available", len(models.Data))

	case http.StatusUnauthorized, http.StatusForbidden:
		return ProviderMisconfigured, "invalid API key"

	case http.StatusNotFound:
		return ProviderWarning, "endpoint not found"

	default:
		return ProviderWarning, fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
}

func (p *HTTPProber) probeOllama(ctx context.Context) (ProviderReadiness, string) {
	// Ollama uses /api/tags for model listing
	req, err := http.NewRequestWithContext(ctx, "GET", p.BaseURL+"/api/tags", nil)
	if err != nil {
		return ProviderMisconfigured, fmt.Sprintf("invalid request: %v", sanitizeError(err, p.APIKey))
	}

	client := &http.Client{Timeout: p.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return ProviderUnavailable, "ollama timeout"
		}
		return ProviderUnavailable, sanitizeError(err, p.APIKey)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ProviderWarning, fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &tags); err != nil {
		return ProviderWarning, "invalid response format"
	}
	if len(tags.Models) == 0 {
		return ProviderWarning, "no models pulled"
	}
	return ProviderReady, fmt.Sprintf("%d models available", len(tags.Models))
}

func (p *HTTPProber) probeLocal(ctx context.Context) (ProviderReadiness, string) {
	// Local providers use OpenAI-compatible /models endpoint
	return p.probeOpenAICompatible(ctx)
}

func sanitizeError(err error, apiKey string) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	// Remove potentially sensitive information
	if apiKey != "" {
		msg = strings.ReplaceAll(msg, apiKey, "***")
	}
	if len(msg) > 100 {
		msg = msg[:100] + "..."
	}
	return msg
}
