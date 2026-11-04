package main

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
)

func TestStandaloneGreetingUsesSmallModelRequest(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{MaxTokens: 4096}, "qwen3.5-4b-FLM")
	// A greeting must not require the tool registry or load workspace skills.
	for _, input := range []string{"yo!", "hello", "hey there"} {
		prompt, tools, maxTokens := app.turnProfile(input)
		if len(prompt) > 1000 || len(tools) != 0 || maxTokens != 256 {
			t.Fatalf("%q sent unnecessary coding context", input)
		}
	}
	app.initTools()
	for _, input := range []string{"yo! fix names.py", "thanks, now run the tests", "continue", "explain this function"} {
		prompt, tools, maxTokens := app.turnProfile(input)
		if len(tools) == 0 || maxTokens != 4096 || !strings.Contains(prompt, "agentic coding assistant") {
			t.Fatalf("work request %q lost the coding profile", input)
		}
	}
}

// TestFitOutputBudget pins the local-window arithmetic: max_tokens is
// part of the context, not a separate allowance. A 4k model with a 4k
// reply budget and a multi-thousand-token prompt would overrun on every
// turn; the reply shrinks to what remains.
func TestFitOutputBudget(t *testing.T) {
	cases := []struct {
		name                            string
		context, prompt, history, asked int
		want                            int
	}{
		{"hosted window keeps request", 1000000, 3000, 2000, 4096, 4096},
		{"reply shrinks to remainder", 8192, 3000, 2000, 4096, 3192},
		{"small local window", 4096, 3000, 500, 4096, 596},
		{"tight history keeps a floor", 4096, 3900, 400, 4096, 256},
		{"unknown window leaves request", 0, 3000, 2000, 4096, 4096},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := fitOutputBudget(tc.context, tc.prompt, tc.history, tc.asked); got != tc.want {
				t.Fatalf("fitOutputBudget(%d, %d, %d, %d) = %d, want %d", tc.context, tc.prompt, tc.history, tc.asked, got, tc.want)
			}
		})
	}
}

func TestGreetingProfileReachesLiveLoop(t *testing.T) {
	app := newHandlerTestApp(t, &config.LayeredConfig{Provider: "lemonade", MaxTokens: 4096}, config.DefaultLemonadeModel)
	client := &recordingClient{respond: "Yo! What are we working on?"}
	app.client = client
	app.loop = agent.NewLoop(client)
	app.runAgentTurn("yo!", tui.NewApp())
	if client.model != config.DefaultLemonadeModel || len(client.tools) != 0 || len(client.system) > 1000 {
		t.Fatal("live greeting request did not use the selected model's small profile")
	}
	if len(app.session.Messages) == 0 {
		t.Fatal("greeting reply was not persisted")
	}
}
