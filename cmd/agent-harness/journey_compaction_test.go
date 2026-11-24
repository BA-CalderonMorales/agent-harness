package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/pkg/messages"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

const (
	journeyWindow      = 4096
	journeySummaryText = "JOURNEY-GOAL carried forward"
	journeyAnswerText  = "Back on track with the goal."
)

// journeyWindowClient behaves like a local server loaded with a small
// context window: a request larger than the window is refused the way
// the HTTP client reports it, and summary requests answer with text
// that carries the goal forward.
type journeyWindowClient struct {
	llm.Client
	mu        sync.Mutex
	refused   int
	summaries int
	answers   int
}

func (c *journeyWindowClient) Stream(_ context.Context, req llm.Request) (<-chan types.LLMEvent, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if (len(req.SystemPrompt)+3)/4+messages.EstimateTokens(req.Messages) > journeyWindow {
		c.refused++
		return nil, errors.New("LLM API error 400: Max length reached!")
	}
	text := journeyAnswerText
	if strings.Contains(req.SystemPrompt, "context summarizer") {
		c.summaries++
		text = fmt.Sprintf("%s; summary part %d", journeySummaryText, c.summaries)
	} else {
		c.answers++
	}
	out := make(chan types.LLMEvent, 3)
	out <- types.LLMMessageStart{ID: "journey-window"}
	out <- types.LLMTextDelta{Delta: text}
	out <- types.LLMMessageStop{StopReason: "stop"}
	close(out)
	return out, nil
}

// journeyLongHistory is about three times the window: ~100 estimated
// tokens per message, opening with the goal.
func journeyLongHistory(count int) []types.Message {
	history := make([]types.Message, 0, count)
	for i := 0; i < count; i++ {
		role := types.RoleUser
		if i%2 == 1 {
			role = types.RoleAssistant
		}
		text := fmt.Sprintf("turn-%03d: %s", i, strings.Repeat("context ", 50))
		if i == 0 {
			text = "JOURNEY-GOAL: keep the small model usable; " + text
		}
		history = append(history, types.Message{
			UUID:    fmt.Sprintf("journey-%03d", i),
			Role:    role,
			Content: []types.ContentBlock{types.TextBlock{Text: text}},
		})
	}
	return history
}

// The journey: a user on a local server with a 4096-token window comes
// back to a long conversation and sends one more message. The turn must
// complete, the screen must say the context was compacted, and the next
// start must resume the compacted conversation, not the overflowing one.
func TestJourneyCompactionOnSmallWindow(t *testing.T) {
	journeyHome(t)
	journeyLocal(t)
	// This journey asserts the record path, which is behind the lab.
	t.Setenv("AH_RECORD_COMPACTION", "1")

	app := bootJourneyApp(t)
	history := journeyLongHistory(120)
	if tokens := messages.EstimateTokens(history); tokens < 3*journeyWindow-1024 {
		t.Fatalf("history is %d tokens, want about three windows", tokens)
	}
	app.session.Messages = history
	app.sessionManager.SetCurrent(app.session)
	if _, err := app.sessionManager.SaveCurrent(); err != nil {
		t.Fatalf("save long session: %v", err)
	}
	sessionID := app.session.ID

	client := &journeyWindowClient{}
	app.client = client
	app.loop.Client = client
	tuiApp := wireJourneyTUI(t, app)
	screen := newJourneyScreen(t, tuiApp, 80, 24)

	// The server advertises the window it loaded the model with; the
	// boot-time discovery lands it on the event loop.
	screen.SendAndPump(tui.ModelContextMsg{Provider: app.config.Provider, Model: app.session.Model, ContextLength: journeyWindow})
	if app.loop.Config.BlockingTokenLimit != journeyWindow {
		t.Fatalf("loop window = %d, want the advertised %d", app.loop.Config.BlockingTokenLimit, journeyWindow)
	}

	screen.Keys("2")
	screen.Keys("i")
	screen.Submit(t, app, "continue")
	screen.waitFor(t, journeyAnswerText)
	screen.waitIdle(t)

	if client.summaries != 0 {
		t.Fatalf("the turn sent %d summarizer request(s); the session record replaces them", client.summaries)
	}
	if client.refused != 0 {
		t.Fatalf("the server refused %d request(s); every request must fit the window", client.refused)
	}
	if client.answers == 0 {
		t.Fatal("the turn never produced an answer")
	}
	screen.AssertFitsWidth(80)
	screen.AssertContains("Context compacted", journeyAnswerText)
	screen.AssertNotContains("Max length reached")

	compacted := len(app.session.Messages)
	if compacted >= len(history) {
		t.Fatalf("session holds %d messages after compaction, want fewer than %d", compacted, len(history))
	}

	// Restart: the latest session resumes, and it is the compacted one.
	restarted := bootJourneyApp(t)
	if restarted.session.ID != sessionID {
		t.Fatalf("restart resumed session %s, want %s", restarted.session.ID, sessionID)
	}
	resumed := restarted.session.Messages
	if len(resumed) != compacted {
		t.Fatalf("resumed session has %d messages, want the compacted %d", len(resumed), compacted)
	}
	if !messagesMention(resumed, "[Session record]") || !messagesMention(resumed, "Goal: JOURNEY-GOAL") {
		t.Fatal("resumed session lost the session record that carries the goal")
	}
	if !messagesMention(resumed, journeyAnswerText) {
		t.Fatal("resumed session lost the turn's answer")
	}
	if tokens := messages.EstimateTokens(resumed); tokens > journeyWindow {
		t.Fatalf("resumed session is %d tokens, larger than the %d window", tokens, journeyWindow)
	}
}

func messagesMention(history []types.Message, fragment string) bool {
	for _, m := range history {
		for _, block := range m.Content {
			if text, ok := block.(types.TextBlock); ok && strings.Contains(text.Text, fragment) {
				return true
			}
		}
	}
	return false
}
