// Package buckets provides domain-specific LoopBase implementations.
package buckets

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/loop"
	"github.com/BA-CalderonMorales/agent-harness/internal/loop/buckets/defaults"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// LoopWeb handles web operations (fetch, search).
// Tools: webfetch, web_search, fetch_url
type LoopWeb struct {
	client         *http.Client
	maxSize        int64
	timeout        time.Duration
	userAgent      string
	allowedSchemes []string
	blockedHosts   []string
}

// NewLoopWeb creates a web bucket.
func NewLoopWeb() *LoopWeb {
	return &LoopWeb{
		client: &http.Client{
			Timeout:       defaults.WebFetchTimeout,
			CheckRedirect: makeRedirectChecker(defaults.WebFetchMaxRedirects),
		},
		maxSize:        defaults.WebFetchMaxSize,
		timeout:        defaults.WebFetchTimeout,
		userAgent:      defaults.WebFetchUserAgent,
		allowedSchemes: defaults.WebAllowedSchemes,
		blockedHosts:   defaults.WebBlockedHosts,
	}
}

// WithTimeout sets the fetch timeout.
func (w *LoopWeb) WithTimeout(d time.Duration) *LoopWeb {
	w.timeout = d
	w.client.Timeout = d
	return w
}

// WithUserAgent sets the user agent.
func (w *LoopWeb) WithUserAgent(ua string) *LoopWeb {
	w.userAgent = ua
	return w
}

// WithMaxSize sets max content size.
func (w *LoopWeb) WithMaxSize(size int64) *LoopWeb {
	w.maxSize = size
	return w
}

// Name returns the bucket identifier.
func (w *LoopWeb) Name() string {
	return "web"
}

// CanHandle determines if this bucket handles the tool.
func (w *LoopWeb) CanHandle(toolName string, input map[string]any) bool {
	switch toolName {
	case "webfetch", "web_fetch", "fetch_url", "websearch", "web_search":
		return true
	}
	return false
}

// Capabilities describes what this bucket can do.
func (w *LoopWeb) Capabilities() loop.BucketCapabilities {
	return loop.BucketCapabilities{
		IsConcurrencySafe: true,
		IsReadOnly:        true,
		IsDestructive:     false,
		ToolNames:         []string{"webfetch", "web_fetch", "fetch_url", "websearch", "web_search"},
		Category:          "web",
	}
}

// Execute runs the web operation.
func (w *LoopWeb) Execute(ctx loop.ExecutionContext) loop.LoopResult {
	switch ctx.ToolName {
	case "webfetch", "web_fetch", "fetch_url":
		return w.handleFetch(ctx)
	case "websearch", "web_search":
		return w.handleSearch(ctx)
	default:
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("unknown_tool", "web bucket doesn't handle: "+ctx.ToolName),
		}
	}
}

// handleFetch fetches a URL.
func (w *LoopWeb) handleFetch(ctx loop.ExecutionContext) loop.LoopResult {
	urlStr, ok := ctx.Input["url"].(string)
	if !ok || urlStr == "" {
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("invalid_input", "url is required"),
		}
	}

	// Parse and validate URL
	u, err := url.Parse(urlStr)
	if err != nil {
		return loop.LoopResult{
			Success: false,
			Error:   loop.WrapError("invalid_url", err),
		}
	}

	// Check scheme
	if !w.isAllowedScheme(u.Scheme) {
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("invalid_scheme", fmt.Sprintf("scheme not allowed: %s", u.Scheme)),
		}
	}

	// Check host
	if w.isBlockedHost(u.Hostname()) {
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("blocked_host", fmt.Sprintf("host blocked: %s", u.Hostname())),
		}
	}

	// Create request
	reqCtx, cancel := context.WithTimeout(ctx.Context, w.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "GET", urlStr, nil)
	if err != nil {
		return loop.LoopResult{
			Success: false,
			Error:   loop.WrapError("request_failed", err),
		}
	}

	req.Header.Set("User-Agent", w.userAgent)

	// Execute request
	resp, err := w.client.Do(req)
	if err != nil {
		return loop.LoopResult{
			Success: false,
			Error:   loop.WrapError("fetch_failed", err),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("http_error", fmt.Sprintf("HTTP %d", resp.StatusCode)),
		}
	}

	// Read content with limit
	content, err := io.ReadAll(io.LimitReader(resp.Body, w.maxSize))
	if err != nil {
		return loop.LoopResult{
			Success: false,
			Error:   loop.WrapError("read_failed", err),
		}
	}

	result := string(content)

	// Truncate if needed
	if len(result) > defaults.WebMaxContentLength {
		result = result[:defaults.WebMaxContentLength] + "\n[content truncated]"
	}

	return loop.LoopResult{
		Success: true,
		Data:    result,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result}},
		}},
	}
}

// handleSearch performs a web search (placeholder for actual search API).
func (w *LoopWeb) handleSearch(ctx loop.ExecutionContext) loop.LoopResult {
	query, ok := ctx.Input["query"].(string)
	if !ok || query == "" {
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("invalid_input", "query is required"),
		}
	}

	// This is a placeholder - real implementation would use search APIs
	// like Google Custom Search, Bing API, Brave Search, etc.
	result := fmt.Sprintf(`Web search for "%s":

[This is a placeholder. To enable real web search, configure a search API provider]

Suggested implementation:
1. Google Custom Search API
2. Bing Search API  
3. Brave Search API
4. SerpAPI
5. Tavily API

The search would return:
- Title and URL for each result
- Snippet/summary
- Optional: Full content fetched and summarized`, query)

	return loop.LoopResult{
		Success: true,
		Data:    result,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result}},
		}},
	}
}

// isAllowedScheme checks if URL scheme is allowed.
func (w *LoopWeb) isAllowedScheme(scheme string) bool {
	for _, s := range w.allowedSchemes {
		if strings.EqualFold(s, scheme) {
			return true
		}
	}
	return false
}

// isBlockedHost checks if host is blocked.
func (w *LoopWeb) isBlockedHost(host string) bool {
	for _, h := range w.blockedHosts {
		if strings.EqualFold(h, host) {
			return true
		}
	}
	return false
}

// makeRedirectChecker creates a redirect checker with max redirects.
func makeRedirectChecker(max int) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= max {
			return fmt.Errorf("too many redirects")
		}
		return nil
	}
}

// FetchResult represents a fetched web page
type FetchResult struct {
	URL         string
	Title       string
	Content     string
	ContentType string
	StatusCode  int
}

// WebSearchResult represents a web search result
type WebSearchResult struct {
	Title   string
	URL     string
	Snippet string
}

// Ensure LoopWeb implements LoopBase
var _ loop.LoopBase = (*LoopWeb)(nil)
