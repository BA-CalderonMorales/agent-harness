package main

import (
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
)

// The sub-agent's query carries the record compaction lab choice of the
// session that started it, on and off.
func TestDelegatedQueryCarriesTheRecordLab(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		want  bool
	}{
		{name: "on", value: "1", want: true},
		{name: "off", value: "", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			journeyHome(t)
			t.Setenv("AH_RECORD_COMPACTION", tc.value)
			app := bootJourneyApp(t)
			got := app.delegatedQueryParams("review the change", "system prompt", nil, nil, tools.Context{})
			if got.RecordCompaction != tc.want {
				t.Fatalf("sub-agent query RecordCompaction = %v, want %v", got.RecordCompaction, tc.want)
			}
		})
	}
}

func TestDelegatedQueryForwardsEffortAndFitsContextBudget(t *testing.T) {
	journeyHome(t)
	app := bootJourneyApp(t)
	app.config.Effort = "high"
	app.config.ContextLength = 4096
	app.config.MaxTokens = 8192

	got := app.delegatedQueryParams("review the change", "system prompt", nil, nil, tools.Context{})
	if got.ReasoningEffort != "high" {
		t.Fatalf("ReasoningEffort = %q, want high", got.ReasoningEffort)
	}
	if got.MaxOutputTokens >= 4096 {
		t.Fatalf("MaxOutputTokens = %d, want bounded below 4096 context window", got.MaxOutputTokens)
	}

	child := app.newConfiguredLoop()
	if child.Config.BlockingTokenLimit != 4096 {
		t.Fatalf("child BlockingTokenLimit = %d, want 4096", child.Config.BlockingTokenLimit)
	}
	if child.Config.StreamIdleTimeout != app.config.StreamIdleTimeout {
		t.Fatalf("child StreamIdleTimeout = %v, want %v", child.Config.StreamIdleTimeout, app.config.StreamIdleTimeout)
	}
}
