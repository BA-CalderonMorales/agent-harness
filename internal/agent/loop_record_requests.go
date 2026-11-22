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
// oldest first, newest few only. The record keeps the goal and a few
// constraint sentences, but a later instruction such as a changed technology
// choice is neither, so without this a compaction would silently drop it.
//
// The message that is the record's goal is skipped; any other user message is
// a request, including the first one left after an earlier compaction. A
// record message from an earlier compaction contributes the requests it
// carried, so they survive every later compaction too. Tool results are
// user-role messages as well; only text blocks count.
func earlierUserRequests(removed []types.Message, goal string) []string {
	var out []string
	for _, m := range removed {
		switch m.Role {
		case types.RoleSystem:
			out = append(out, carriedIn(m)...)
			continue
		case types.RoleUser:
		default:
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
		if line == "" || isGoal(line, goal) {
			continue
		}
		out = append(out, clipRequest(line))
	}
	if len(out) > maxCarriedRequests {
		out = out[len(out)-maxCarriedRequests:]
	}
	return out
}

// isGoal reports whether a whitespace-collapsed message is the one the record
// took its goal from. The record clips a long goal and ends it with "…".
func isGoal(line, goal string) bool {
	if goal == "" {
		return false
	}
	if line == goal {
		return true
	}
	head, cut := strings.CutSuffix(goal, "…")
	return cut && strings.HasPrefix(line, strings.TrimRight(head, " "))
}

// carriedIn returns the requests an earlier record message carried.
func carriedIn(m types.Message) []string {
	var out []string
	for _, b := range m.Content {
		tb, ok := b.(types.TextBlock)
		if !ok || !strings.HasPrefix(tb.Text, recordSummaryLead) {
			continue
		}
		_, section, found := strings.Cut(tb.Text, carriedRequestsTitle)
		if !found {
			continue
		}
		for _, line := range strings.Split(section, "\n") {
			if req, ok := strings.CutPrefix(line, "- "); ok && req != "" {
				out = append(out, req)
			}
		}
	}
	return out
}

func clipRequest(line string) string {
	if r := []rune(line); len(r) > maxCarriedRequestRunes {
		return string(r[:maxCarriedRequestRunes-1]) + "…"
	}
	return line
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
