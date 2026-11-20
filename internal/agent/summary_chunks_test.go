package agent

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestChunkTranscriptKeepsOrderAndStaysWithinBudget(t *testing.T) {
	var lines []string
	for i := 0; i < 50; i++ {
		lines = append(lines, fmt.Sprintf("user: line-%02d %s", i, strings.Repeat("x", 90)))
	}
	const budget = 120
	chunks := chunkTranscript(lines, budget)
	if len(chunks) < 2 {
		t.Fatalf("chunks = %d, want the transcript split", len(chunks))
	}
	for i, chunk := range chunks {
		if tokens := estimateTextTokens(chunk); tokens > budget {
			t.Fatalf("chunk %d holds %d estimated tokens, budget %d", i, tokens, budget)
		}
	}
	if joined := strings.Join(chunks, ""); joined != strings.Join(lines, "\n")+"\n" {
		t.Fatal("chunks do not reassemble into the original transcript in order")
	}
}

func TestChunkTranscriptTrimsALineLargerThanTheBudget(t *testing.T) {
	huge := "tool: HEAD-MARK " + strings.Repeat("y", 10000) + " TAIL-MARK"
	chunks := chunkTranscript([]string{"user: before", huge, "user: after"}, 200)
	var trimmed string
	for _, chunk := range chunks {
		if tokens := estimateTextTokens(chunk); tokens > 200 {
			t.Fatalf("a chunk holds %d estimated tokens, budget 200", tokens)
		}
		if strings.Contains(chunk, "HEAD-MARK") {
			trimmed = chunk
		}
	}
	if !strings.Contains(trimmed, "TAIL-MARK") || !strings.Contains(trimmed, "characters omitted") {
		t.Fatalf("the oversized line lost its head, tail, or omission marker: %q", trimmed)
	}
	if !strings.Contains(strings.Join(chunks, ""), "user: after") {
		t.Fatal("the line after the oversized one was dropped")
	}
}

func TestChunkTranscriptOfNothingIsEmpty(t *testing.T) {
	if chunks := chunkTranscript(nil, 100); len(chunks) != 0 {
		t.Fatalf("chunks = %d, want none", len(chunks))
	}
}

func TestTruncateMiddleKeepsBoundsAndCharacters(t *testing.T) {
	text := strings.Repeat("é", 500) + "MIDDLE" + strings.Repeat("ü", 500)
	got := truncateMiddle(text, 300)
	if len(got) > 300 {
		t.Fatalf("truncated length = %d bytes, want at most 300", len(got))
	}
	if !utf8.ValidString(got) {
		t.Fatal("truncation split a multi-byte character")
	}
	if !strings.HasPrefix(got, "é") || !strings.HasSuffix(got, "ü") {
		t.Fatalf("truncation lost the head or tail: %q", got)
	}
	if short := "fits"; truncateMiddle(short, 10) != short {
		t.Fatal("text within the limit must come back unchanged")
	}
}

func TestSummaryChunkBudgetLeavesRoomInAFourKWindow(t *testing.T) {
	budget := summaryChunkBudget(4096)
	if budget < minSummaryChunkTokens || budget >= 4096 {
		t.Fatalf("budget for a 4096-token window = %d, want a usable part of the window", budget)
	}
	if small := summaryChunkBudget(100); small >= minSummaryChunkTokens {
		t.Fatalf("budget for a 100-token window = %d, want below the chunking minimum", small)
	}
}
