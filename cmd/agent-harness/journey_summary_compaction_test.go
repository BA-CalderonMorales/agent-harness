package main

import (
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
)

// The journey with the record compaction lab at its default (off): the same
// long session on a small window must compact the pre-record way, with the
// model writing the summary, through the real turn wiring.
func TestJourneySummaryCompactionWhenRecordLabIsOff(t *testing.T) {
	journeyHome(t)
	journeyLocal(t)
	t.Setenv("AH_RECORD_COMPACTION", "")

	app := bootJourneyApp(t)
	if app.config.LabEnabled(config.LabRecordCompaction) {
		t.Fatal("record compaction is on by default; the lab must default off")
	}
	history := journeyLongHistory(120)
	app.session.Messages = history
	app.sessionManager.SetCurrent(app.session)
	if _, err := app.sessionManager.SaveCurrent(); err != nil {
		t.Fatalf("save long session: %v", err)
	}

	client := &journeyWindowClient{}
	app.client = client
	app.loop.Client = client
	tuiApp := wireJourneyTUI(t, app)
	screen := newJourneyScreen(t, tuiApp, 80, 24)
	screen.SendAndPump(tui.ModelContextMsg{Provider: app.config.Provider, Model: app.session.Model, ContextLength: journeyWindow})

	screen.Keys("2")
	screen.Keys("i")
	screen.Submit(t, app, "continue")
	screen.waitFor(t, journeyAnswerText)
	screen.waitIdle(t)

	if client.summaries == 0 {
		t.Fatal("the turn sent no summarizer request; with the lab off the model must write the summary")
	}
	if client.refused != 0 {
		t.Fatalf("the server refused %d request(s); every request must fit the window", client.refused)
	}
	if !messagesMention(app.session.Messages, "[Earlier conversation summarized]") {
		t.Fatal("the compacted session does not hold the model-written summary")
	}
	if messagesMention(app.session.Messages, "[Session record]") {
		t.Fatal("the compacted session holds the session record with the lab off")
	}
}
