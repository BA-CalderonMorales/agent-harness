package llm

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// TestLiveLemonadeSmoke runs one real short turn against a local
// Lemonade server through the real HTTPClient. It is opt-in (make
// smoke-lemonade sets AH_LIVE_LEMONADE=1) and never runs in CI.
//
// Overrides: LEMONADE_BASE_URL, LEMONADE_MODEL.
func TestLiveLemonadeSmoke(t *testing.T) {
	if os.Getenv("AH_LIVE_LEMONADE") != "1" {
		t.Skip("set AH_LIVE_LEMONADE=1 (make smoke-lemonade) to run against a local Lemonade server")
	}
	baseURL := os.Getenv("LEMONADE_BASE_URL")
	if baseURL == "" {
		baseURL = config.DefaultLemonadeEndpoint
	}
	client := NewHTTPClientWithBaseURLTimeout("lemonade", "", baseURL, 4*time.Minute)

	infos, err := client.ListModelsDetailed()
	if err != nil {
		t.Fatalf("GET /models failed (is Lemonade running at %s?): %v", baseURL, err)
	}
	if len(infos) == 0 {
		t.Fatal("Lemonade advertises no models")
	}
	model := os.Getenv("LEMONADE_MODEL")
	var window int
	for _, info := range infos {
		if model == "" || info.ID == model {
			if model == "" {
				model = info.ID
			}
			window = info.ContextLength
			break
		}
	}
	if window < 1024 || window > 4<<20 {
		t.Fatalf("model %q context_length = %d, want a sane window (1024 to 4194304)", model, window)
	}
	t.Logf("model %s, context_length %d", model, window)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	events, err := client.Stream(ctx, Request{
		Model:     model,
		MaxTokens: 64,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.TextBlock{Text: "Reply with exactly: ready"}},
		}},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	got := observeSSEEvents(drainSSEEvents(events))
	if len(got.errors) > 0 {
		t.Fatalf("stream error: %v", got.errors[0])
	}
	answer := strings.TrimSpace(strings.Join(got.text, ""))
	if answer == "" {
		t.Fatal("empty answer from Lemonade")
	}
	if len(got.stops) != 1 {
		t.Fatalf("stops = %#v, want exactly one", got.stops)
	}
	t.Logf("answer %q, stop %q, usage %+v", answer, got.stops[0].StopReason, got.stops[0].Usage)
}
