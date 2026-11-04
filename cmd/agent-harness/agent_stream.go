package main

import (
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// streamedTextMatches reports whether a message's prose repeats exactly
// what was already streamed for this request. Providers emit deltas and
// then the assembled message; the TUI must not show the answer twice. An
// empty stream falls through to the message blocks, so a provider that
// never streams deltas still renders its answer.
func streamedTextMatches(message types.Message, streamed string) bool {
	var text strings.Builder
	for _, block := range message.Content {
		if tb, ok := block.(types.TextBlock); ok {
			text.WriteString(tb.Text)
		}
	}
	return text.Len() > 0 && text.String() == streamed
}
