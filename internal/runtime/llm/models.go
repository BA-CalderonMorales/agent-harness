package llm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

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
// Results are cached for 5 minutes. Local OpenAI-compatible servers
// (llama.cpp, ollama, demo mocks) serve /v1/models too, so they are
// probed like any other provider; a server that is down or lacks the
// endpoint fails the probe and callers fall back to the static catalog.
func (c *HTTPClient) ListModels() ([]string, error) {
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
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, sanitizeError(fmt.Errorf("%s", string(body)), c.APIKey))
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
