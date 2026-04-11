// Package buckets provides domain-specific LoopBase implementations.
package buckets

import (
	"fmt"
	"strings"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/loop"
	"github.com/BA-CalderonMorales/agent-harness/internal/loop/buckets/defaults"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// LoopTranscript handles conversation transcript operations.
// Tools: search_transcript, summarize_transcript
type TranscriptBucket struct {
	maxHistory int
	transcript []TranscriptEntry
}

// TranscriptEntry represents a single message in the transcript.
type TranscriptEntry struct {
	Timestamp time.Time
	Role      string
	Content   string
	ToolCalls []ToolCallInfo
}

// ToolCallInfo represents a tool invocation.
type ToolCallInfo struct {
	ToolName string
	Input    map[string]any
	Result   string
}

// NewLoopTranscript creates a transcript bucket.
func Transcript() *TranscriptBucket {
	return &TranscriptBucket{
		maxHistory: defaults.TranscriptMaxHistoryDefault,
		transcript: make([]TranscriptEntry, 0),
	}
}

// WithMaxHistory sets the maximum history to search.
func (t *TranscriptBucket) WithMaxHistory(n int) *TranscriptBucket {
	t.maxHistory = n
	return t
}

// Name returns the bucket identifier.
func (t *TranscriptBucket) Name() string {
	return "transcript"
}

// CanHandle determines if this bucket handles the tool.
func (t *TranscriptBucket) CanHandle(toolName string, input map[string]any) bool {
	switch toolName {
	case "search_transcript", "transcript_search", "summarize_transcript", "transcript_summary":
		return true
	}
	return false
}

// Capabilities describes what this bucket can do.
func (t *TranscriptBucket) Capabilities() loop.BucketCapabilities {
	return loop.BucketCapabilities{
		IsConcurrencySafe: true,
		IsReadOnly:        true,
		IsDestructive:     false,
		ToolNames:         []string{"search_transcript", "transcript_search", "summarize_transcript"},
		Category:          "transcript",
	}
}

// Execute runs the transcript operation.
func (t *TranscriptBucket) Execute(ctx loop.ExecutionContext) loop.LoopResult {
	switch ctx.ToolName {
	case "search_transcript", "transcript_search":
		return t.handleSearch(ctx)
	case "summarize_transcript", "transcript_summary":
		return t.handleSummarize(ctx)
	default:
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("unknown_tool", "transcript bucket doesn't handle: "+ctx.ToolName),
		}
	}
}

// handleSearch searches the conversation transcript.
func (t *TranscriptBucket) handleSearch(ctx loop.ExecutionContext) loop.LoopResult {
	query, ok := ctx.Input["query"].(string)
	if !ok || query == "" {
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("invalid_input", "query is required"),
		}
	}

	// Use provided messages as transcript source
	source := ctx.Messages
	if len(source) == 0 {
		return loop.LoopResult{
			Success: true,
			Data:    "(no messages to search)",
			Messages: []types.Message{{
				Role:    types.RoleUser,
				Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: "(no messages to search)"}},
			}},
		}
	}

	// Limit search scope
	startIdx := 0
	if len(source) > t.maxHistory {
		startIdx = len(source) - t.maxHistory
	}

	// Search for matches
	var matches []string
	for i := startIdx; i < len(source); i++ {
		msg := source[i]
		content := extractTextContent(msg)

		if strings.Contains(strings.ToLower(content), strings.ToLower(query)) {
			match := fmt.Sprintf("[%d] %s: %s", i, msg.Role, truncate(content, 200))
			matches = append(matches, match)
		}
	}

	var result string
	if len(matches) == 0 {
		result = fmt.Sprintf("No matches found for '%s'", query)
	} else {
		result = fmt.Sprintf("Found %d matches for '%s':\n%s", len(matches), query, strings.Join(matches, "\n"))
	}

	return loop.LoopResult{
		Success: true,
		Data:    matches,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result}},
		}},
	}
}

// handleSummarize creates a summary of the transcript.
func (t *TranscriptBucket) handleSummarize(ctx loop.ExecutionContext) loop.LoopResult {
	// Use provided messages
	source := ctx.Messages
	if len(source) == 0 {
		return loop.LoopResult{
			Success: true,
			Data:    "(no messages to summarize)",
			Messages: []types.Message{{
				Role:    types.RoleUser,
				Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: "(no messages to summarize)"}},
			}},
		}
	}

	// Limit scope
	startIdx := 0
	if len(source) > t.maxHistory {
		startIdx = len(source) - t.maxHistory
	}

	// Generate simple summary
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Conversation Summary (%d messages):\n\n", len(source)-startIdx))

	// Count by role
	roleCount := make(map[string]int)
	toolCount := 0
	for i := startIdx; i < len(source); i++ {
		roleCount[string(source[i].Role)]++
		// Count tool uses
		for _, block := range source[i].Content {
			if _, ok := block.(types.ToolUseBlock); ok {
				toolCount++
			}
		}
	}

	summary.WriteString("Message counts:\n")
	for role, count := range roleCount {
		summary.WriteString(fmt.Sprintf("  %s: %d\n", role, count))
	}
	if toolCount > 0 {
		summary.WriteString(fmt.Sprintf("  tool calls: %d\n", toolCount))
	}

	// List topics (keywords from user messages)
	topics := extractTopics(source[startIdx:])
	if len(topics) > 0 {
		summary.WriteString(fmt.Sprintf("\nKey topics: %s", strings.Join(topics, ", ")))
	}

	result := summary.String()

	return loop.LoopResult{
		Success: true,
		Data:    result,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result}},
		}},
	}
}

// AddEntry adds an entry to the transcript.
func (t *TranscriptBucket) AddEntry(role, content string) {
	entry := TranscriptEntry{
		Timestamp: time.Now(),
		Role:      role,
		Content:   content,
	}
	t.transcript = append(t.transcript, entry)

	// Trim if too large
	if len(t.transcript) > t.maxHistory {
		t.transcript = t.transcript[len(t.transcript)-t.maxHistory:]
	}
}

// GetTranscript returns the full transcript.
func (t *TranscriptBucket) GetTranscript() []TranscriptEntry {
	result := make([]TranscriptEntry, len(t.transcript))
	copy(result, t.transcript)
	return result
}

// Clear resets the transcript.
func (t *TranscriptBucket) Clear() {
	t.transcript = make([]TranscriptEntry, 0)
}

// Helper functions

func extractTextContent(msg types.Message) string {
	var parts []string
	for _, block := range msg.Content {
		switch b := block.(type) {
		case types.TextBlock:
			parts = append(parts, b.Text)
		case types.ToolUseBlock:
			parts = append(parts, fmt.Sprintf("[Tool: %s]", b.Name))
		case types.ToolResultBlock:
			parts = append(parts, fmt.Sprintf("[Result: %v]", b.Content))
		}
	}
	return strings.Join(parts, " ")
}

// IsStopWord checks if word is a stop word.
func IsStopWord(word string) bool {
	_, ok := defaults.TranscriptStopWords[strings.ToLower(word)]
	return ok
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func extractTopics(msgs []types.Message) []string {
	// Simple keyword extraction from user messages
	keywordCount := make(map[string]int)

	for _, msg := range msgs {
		if msg.Role != types.RoleUser {
			continue
		}
		content := extractTextContent(msg)
		words := strings.Fields(strings.ToLower(content))
		for _, w := range words {
			w = strings.TrimRight(w, ",.!?;:")
			if len(w) > 4 && !IsStopWord(w) {
				keywordCount[w]++
			}
		}
	}

	// Get top keywords
	var topics []string
	for kw, count := range keywordCount {
		if count >= defaults.TranscriptMinTopicFreq && len(topics) < defaults.TranscriptMaxTopics {
			topics = append(topics, kw)
		}
	}

	return topics
}

// Ensure LoopTranscript implements LoopBase
var _ loop.LoopBase = (*TranscriptBucket)(nil)
