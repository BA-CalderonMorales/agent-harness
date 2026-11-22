package agent

import (
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/record"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

const (
	// maxCarriedRequests is how many of the removed user requests ride along
	// with the record, newest first kept.
	maxCarriedRequests = 4

	// maxCarriedRequestRunes clips each carried request to one short line.
	maxCarriedRequestRunes = 120

	carriedRequestsTitle = "\nEarlier user requests:"
)

// earlierUserRequests lists the user's own words in the removed history,
// oldest first, newest few only. The first user message is the record's goal
// and is skipped. The record keeps the goal and a few constraint sentences,
// but a later instruction such as a changed technology choice is neither, so
// without this a compaction would silently drop it. Tool results are user-role
// messages too; only text blocks count.
func earlierUserRequests(removed []types.Message) []string {
	var out []string
	seenFirst := false
	for _, m := range removed {
		if m.Role != types.RoleUser {
			continue
		}
		var text strings.Builder
		for _, b := range m.Content {
			if tb, ok := b.(types.TextBlock); ok {
				text.WriteString(tb.Text)
				text.WriteByte(' ')
			}
		}
		line := strings.Join(strings.Fields(text.String()), " ")
		if line == "" {
			continue
		}
		if !seenFirst {
			seenFirst = true
			continue
		}
		if r := []rune(line); len(r) > maxCarriedRequestRunes {
			line = string(r[:maxCarriedRequestRunes-1]) + "…"
		}
		out = append(out, line)
	}
	if len(out) > maxCarriedRequests {
		out = out[len(out)-maxCarriedRequests:]
	}
	return out
}

func requestsSection(reqs []string) string {
	if len(reqs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(carriedRequestsTitle)
	for _, r := range reqs {
		b.WriteString("\n- ")
		b.WriteString(r)
	}
	return b.String()
}

// recordBody renders the record and the carried requests into one body of at
// most tokens estimated tokens. The oldest carried requests give way first
// when the record needs the room; the record is never starved below the
// minimum render for them.
func recordBody(rec record.Record, reqs []string, tokens int) string {
	for n := len(reqs); n >= 0; n-- {
		section := requestsSection(reqs[len(reqs)-n:])
		room := tokens - estimateText(section)
		if n > 0 && room < minRecordSummaryTokens {
			continue
		}
		return record.Render(rec, room) + section
	}
	return ""
}
