package main

import (
	"fmt"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/feedback"
)

const feedbackUsage = "Usage: /feedback [bug|annoying|missing|other] <text>"

// registerFeedbackCommand wires /feedback into the registry behind the
// feedback Labs flag. The command never starts a model turn: its result
// goes to the transcript and the record goes to a local file.
func registerFeedbackCommand(app *App) {
	app.cmdRegistry.RegisterLabs("feedback",
		"Save a friction note locally (usage: /feedback [bug|annoying|missing|other] <text>)",
		config.LabFeedback,
		func() bool { return app.config.LabEnabled(config.LabFeedback) },
		func(args string) (string, error) {
			return app.saveFeedback(args)
		})
}

// saveFeedback appends one report built from the command argument and the
// current session's identity. Only the fields in feedback.Record are written.
func (app *App) saveFeedback(args string) (string, error) {
	category, text := feedback.ParseArgs(args)
	if text == "" {
		return feedbackUsage, nil
	}
	sessionID := ""
	if app.session != nil {
		sessionID = app.session.ID
	}
	rec := feedback.Record{
		Time:           time.Now().UTC().Format(time.RFC3339),
		SessionID:      sessionID,
		Provider:       app.config.Provider,
		Model:          app.config.Model,
		HarnessVersion: Version,
		Category:       category,
		Text:           text,
	}
	path := config.DataFeedback()
	if err := feedback.Append(path, rec); err != nil {
		return "", err
	}
	return fmt.Sprintf("Saved. Thanks. (%s)", path), nil
}
