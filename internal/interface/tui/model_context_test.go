package tui

import "testing"

func TestModelContextMsgCallsHandler(t *testing.T) {
	app := NewApp()
	var gotProvider, gotModel string
	var gotContext int
	app.SetModelContextHandler(func(provider, model string, contextLength int) {
		gotProvider, gotModel, gotContext = provider, model, contextLength
	})

	app.Update(ModelContextMsg{Provider: "openrouter", Model: "nvidia/nemotron-3.5-lightning:free", ContextLength: 1000000})

	if gotProvider != "openrouter" || gotModel != "nvidia/nemotron-3.5-lightning:free" || gotContext != 1000000 {
		t.Fatalf("handler got (%q, %q, %d), want the discovered window", gotProvider, gotModel, gotContext)
	}
}
