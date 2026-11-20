package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

const (
	// summaryOutputTokens is the reply room each summarizer request asks
	// for, and the most a running summary carries into the next part.
	summaryOutputTokens = 512

	// summaryWindowPercent is the share of the context window a summarizer
	// request may fill. Four characters per token undercounts code and
	// non-English text, so the rest is a safety margin.
	summaryWindowPercent = 85

	// minSummaryChunkTokens is the smallest transcript part worth sending.
	// A window too small to fit it cannot hold a useful summary either.
	minSummaryChunkTokens = 64
)

const summarySystemPrompt = "You are a context summarizer producing a durable continuation record. Be concise, structured, and loss-averse. Preserve the original goal, constraints, decisions, approvals, pending tasks, plans, changed files, tool outcomes or receipts, errors, and verification state."

const summaryInstructions = "Create a durable handoff from the conversation prefix below.\n" +
	"Preserve the original user goal, constraints, decisions and approvals, pending work, plans, files changed, tool findings or receipts, errors, and verification status.\n" +
	"State uncertainty explicitly. Put the original goal and unresolved work first.\n\n"

const summaryCarryForward = "Summary of the earlier part of this prefix:\n%s\n\nUpdate that summary with the next part of the prefix below, keeping everything it records.\n\n"

// estimateTextTokens applies the same four-characters-per-token estimate
// as messages.EstimateTokens to a single string.
func estimateTextTokens(text string) int {
	return (len(text) + 3) / 4
}

// summaryChunkBudget is how many estimated tokens of transcript one
// summarizer request can carry in a window: the window share, less the
// fixed prompt, the reply, and a running summary from the previous part.
func summaryChunkBudget(window int) int {
	overhead := estimateTextTokens(summarySystemPrompt) +
		estimateTextTokens(summaryInstructions) +
		estimateTextTokens(summaryCarryForward)
	return window*summaryWindowPercent/100 - overhead - 2*summaryOutputTokens
}

// renderTranscriptLines turns messages into the one-line-per-message form
// the summarizer reads. Reasoning is left out of the durable record.
func renderTranscriptLines(msgs []types.Message) []string {
	lines := make([]string, 0, len(msgs))
	for _, msg := range msgs {
		var b strings.Builder
		fmt.Fprintf(&b, "%s: ", msg.Role)
		for _, block := range msg.Content {
			switch blk := block.(type) {
			case types.TextBlock:
				b.WriteString(blk.Text)
			case types.ThinkingBlock:
				b.WriteString("[reasoning omitted from durable summary input]")
			case types.ToolUseBlock:
				input, _ := json.Marshal(blk.Input)
				fmt.Fprintf(&b, "[tool id=%s name=%s input=%s]", blk.ID, blk.Name, input)
			case types.ToolResultBlock:
				fmt.Fprintf(&b, "[tool result id=%s error=%t: %s]", blk.ToolUseID, blk.IsError, blk.Content)
			}
		}
		lines = append(lines, b.String())
	}
	return lines
}

// chunkTranscript groups lines, in order, into chunks whose estimated size
// stays within budget tokens. A line that alone exceeds the budget keeps
// its head and tail, so no chunk can outgrow the window.
func chunkTranscript(lines []string, budget int) []string {
	var chunks []string
	var current strings.Builder
	currentTokens := 0
	for _, line := range lines {
		if estimateTextTokens(line+"\n") > budget {
			line = truncateMiddle(line, budget*4-1)
		}
		lineTokens := estimateTextTokens(line + "\n")
		if currentTokens > 0 && currentTokens+lineTokens > budget {
			chunks = append(chunks, current.String())
			current.Reset()
			currentTokens = 0
		}
		current.WriteString(line)
		current.WriteString("\n")
		currentTokens += lineTokens
	}
	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}
	return chunks
}

// truncateMiddle shortens text to at most maxChars bytes by keeping its
// head and tail around a marker that says how much was dropped.
func truncateMiddle(text string, maxChars int) string {
	if len(text) <= maxChars {
		return text
	}
	marker := fmt.Sprintf(" …[%d characters omitted]… ", len(text)-maxChars)
	keep := maxChars - len(marker)
	if keep < 2 {
		return text[:maxChars]
	}
	head := keep / 2
	tail := len(text) - (keep - head)
	// Cut on character boundaries so the summarizer never sees a split
	// multi-byte character.
	for head > 0 && !utf8.RuneStart(text[head]) {
		head--
	}
	for tail < len(text) && !utf8.RuneStart(text[tail]) {
		tail++
	}
	return text[:head] + marker + text[tail:]
}
