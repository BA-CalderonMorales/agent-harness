package agent

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/core/record"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// seedRecord picks the record a query starts from: the caller's, else one
// folded from the transcript the query was given, else the zero Record.
func seedRecord(params QueryParams) record.Record {
	if !params.Record.IsZero() {
		return params.Record
	}
	if len(params.Messages) > 0 {
		return record.Fold(params.Messages)
	}
	return record.Record{}
}

// add appends messages to the history and folds each into the loop's copy of
// the record, so the copy never lags the messages. Compaction rewrites the
// history but never the record.
func (s *loopState) add(msgs ...types.Message) {
	for _, m := range msgs {
		s.messages = append(s.messages, m)
		s.record = record.Apply(s.record, m)
	}
}
