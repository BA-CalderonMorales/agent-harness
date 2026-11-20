package main

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/core/record"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/state"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// recordMessage keeps the session's structured record current. Call it right
// after the message was added to the session and before the session is saved,
// so the record line and the message land in the same write. The record is
// evidence only: nothing reads it back into a request yet.
func (app *App) recordMessage(msg types.Message) {
	s := app.session
	if s == nil {
		return
	}
	if s.Record.IsZero() && len(s.Messages) > 1 {
		// A session that predates the record (or one whose file had no record
		// line) rebuilds from its whole transcript, which already holds msg.
		s.Record = record.Fold(s.Messages)
		return
	}
	s.Record = record.Apply(s.Record, msg)
}

// rebuildRecord gives a session loaded from a file without a record line the
// record its transcript implies. A session that has one is left alone.
func rebuildRecord(s *state.Session) {
	if s != nil && s.Record.IsZero() && len(s.Messages) > 0 {
		s.Record = record.Fold(s.Messages)
	}
}
