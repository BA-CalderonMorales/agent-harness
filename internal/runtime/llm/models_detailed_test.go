package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestListModelsDetailedCarriesAdvertisedContext pins the budget-bar
// contract: the live /v1/models context_length flows into ModelInfo so
// the context bar can prefer the provider's word over the catalog.
func TestListModelsDetailedCarriesAdvertisedContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "m/big", "context_length": 1048576},
				{"id": "m/silent"}, // no advertised context
			},
		})
	}))
	defer srv.Close()

	c := NewHTTPClientWithBaseURL("fireworks", "key", srv.URL)
	infos, err := c.ListModelsDetailed()
	if err != nil {
		t.Fatalf("ListModelsDetailed() error = %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("got %d models, want 2", len(infos))
	}
	if infos[0].ID != "m/big" || infos[0].ContextLength != 1048576 {
		t.Fatalf("m/big = %+v, want context 1048576", infos[0])
	}
	if infos[1].ID != "m/silent" || infos[1].ContextLength != 0 {
		t.Fatalf("m/silent = %+v, want zero context", infos[1])
	}

	// The plain ID list shares the same cache.
	ids, err := c.ListModels()
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(ids) != 2 || ids[0] != "m/big" {
		t.Fatalf("ListModels() = %v", ids)
	}
}
