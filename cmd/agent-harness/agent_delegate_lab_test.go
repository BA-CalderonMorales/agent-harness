package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
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

	// Inherited model keeps the parent's 4096 window.
	inheritedCtx := tools.Context{Options: tools.Options{MainLoopModel: app.session.Model}}
	got := app.delegatedQueryParams("review the change", "system prompt", nil, nil, inheritedCtx)
	if got.ReasoningEffort != "high" {
		t.Fatalf("ReasoningEffort = %q, want high", got.ReasoningEffort)
	}
	if got.MaxOutputTokens >= 4096 {
		t.Fatalf("MaxOutputTokens = %d, want bounded below 4096 context window", got.MaxOutputTokens)
	}

	// Specialist child model resolves its own larger window instead of being
	// clamped to the parent's 4096 window.
	specialistCtx := tools.Context{Options: tools.Options{MainLoopModel: "gpt-4o"}}
	gotSpecialist := app.delegatedQueryParams("review the change", "system prompt", nil, nil, specialistCtx)
	if gotSpecialist.MaxOutputTokens != 8192 {
		t.Fatalf("specialist MaxOutputTokens = %d, want 8192 under gpt-4o's 128k window", gotSpecialist.MaxOutputTokens)
	}
	if gotWindow := app.delegatedContextLength("gpt-4o"); gotWindow != 128000 {
		t.Fatalf("delegatedContextLength(gpt-4o) = %d, want 128000", gotWindow)
	}
}

func TestDelegatedAgentUsesChildModelContextWindowWhenParentIsSmaller(t *testing.T) {
	journeyHome(t)
	app := bootJourneyApp(t)

	// Install a specialist agent definition whose model has a 128k context
	// window while the parent session has a tiny 100-token window smaller than
	// the child's fixed system prompt.
	agentDir := filepath.Join(app.cwd, ".agent-harness", "agents")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	def := "---\nname: specialist\ndescription: Large-window reviewer\nmodel: gpt-4o\n---\nYou are a careful specialist reviewer.\n"
	if err := os.WriteFile(filepath.Join(agentDir, "specialist.md"), []byte(def), 0o600); err != nil {
		t.Fatal(err)
	}

	app.config.ContextLength = 100
	app.config.MaxTokens = 4096
	app.client = staticDelegateClient{reply: "specialist ok"}
	app.syncLoopConfig(app.loop)

	out, err := app.runDelegatedAgent(context.Background(), "review this", "specialist", tools.Context{
		AbortController: context.Background(),
	}, nil)
	if err != nil {
		t.Fatalf("runDelegatedAgent() failed with small parent window: %v", err)
	}
	if out != "specialist ok" {
		t.Fatalf("runDelegatedAgent() = %q, want %q", out, "specialist ok")
	}
}

type staticDelegateClient struct{ reply string }

func (c staticDelegateClient) Stream(_ context.Context, _ llm.Request) (<-chan types.LLMEvent, error) {
	ch := make(chan types.LLMEvent, 3)
	ch <- types.LLMMessageStart{ID: "child-1"}
	ch <- types.LLMTextDelta{Delta: c.reply}
	ch <- types.LLMMessageStop{StopReason: "stop", Model: "gpt-4o"}
	close(ch)
	return ch, nil
}
