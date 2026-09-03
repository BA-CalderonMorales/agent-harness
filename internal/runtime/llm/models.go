package llm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// ModelInfo is one live /v1/models entry: the model ID plus whatever
// the provider advertises about it. ContextLength is 0 when the
// endpoint does not say — callers fall back to the static catalog.
type ModelInfo struct {
	ID            string
	ContextLength int
}

// modelCache caches fetched model lists per base URL.
type modelCache struct {
	models    []ModelInfo
	fetchedAt time.Time
}

var (
	modelCacheMu  sync.Mutex
	modelCaches   = make(map[string]*modelCache)
	modelCacheTTL = 5 * time.Minute
)

// fetchModels performs the /v1/models request and decodes the entries
// with every advertised field the budget bar cares about.
func (c *HTTPClient) fetchModels() ([]ModelInfo, error) {
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
			ID            string `json:"id"`
			ContextLength int    `json:"context_length"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	models := make([]ModelInfo, 0, len(result.Data))
	for _, m := range result.Data {
		if m.ID != "" {
			models = append(models, ModelInfo{ID: m.ID, ContextLength: m.ContextLength})
		}
	}
	return models, nil
}

// listModelsCached returns the cached model list for the client's base
// URL, fetching and caching when stale.
func (c *HTTPClient) listModelsCached() ([]ModelInfo, error) {
	modelCacheMu.Lock()
	cache, ok := modelCaches[c.BaseURL]
	if ok && time.Since(cache.fetchedAt) < modelCacheTTL {
		modelCacheMu.Unlock()
		return cache.models, nil
	}
	modelCacheMu.Unlock()

	models, err := c.fetchModels()
	if err != nil {
		return nil, err
	}

	modelCacheMu.Lock()
	modelCaches[c.BaseURL] = &modelCache{models: models, fetchedAt: time.Now()}
	modelCacheMu.Unlock()

	return models, nil
}

// ListModels fetches available model IDs from the provider API.
// Results are cached for 5 minutes. Local OpenAI-compatible servers
// (llama.cpp, ollama, demo mocks) serve /v1/models too, so they are
// probed like any other provider; a server that is down or lacks the
// endpoint fails the probe and callers fall back to the static catalog.
func (c *HTTPClient) ListModels() ([]string, error) {
	infos, err := c.listModelsCached()
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(infos))
	for _, info := range infos {
		ids = append(ids, info.ID)
	}
	return ids, nil
}

// ListModelsDetailed fetches the model list with advertised metadata
// (same cache as ListModels). ContextLength is the provider-advertised
// context window in tokens, 0 when unadvertised.
func (c *HTTPClient) ListModelsDetailed() ([]ModelInfo, error) {
	return c.listModelsCached()
}
