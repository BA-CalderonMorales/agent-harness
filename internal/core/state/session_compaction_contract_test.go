package state

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func TestSessionCompactionPreservesDurableGoalAndSessionState(t *testing.T) {
	session := NewSession("test-model")
	session.Persona = "scientist"
	session.PlanMode = true
	session.Messages = []types.Message{
		{UUID: "goal", Role: types.RoleUser, Content: []types.ContentBlock{types.TextBlock{Text: "ORIGINAL-GOAL: stabilize the production loop"}}},
		{UUID: "constraint", Role: types.RoleSystem, Content: []types.ContentBlock{types.TextBlock{Text: "CONSTRAINT: preserve the canonical loop"}}},
		{UUID: "tool", Role: types.RoleUser, Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: "call-1", Content: "VERIFICATION: race reproduced"}}},
		{UUID: "pending", Role: types.RoleUser, Content: []types.ContentBlock{types.TextBlock{Text: "PENDING-WORK: repair permissions, executor, SSE, and compaction"}}},
	}

	var summarized []types.Message
	result := session.Compact(CompactionConfig{
		MaxMessages:        2,
		MaxEstimatedTokens: 1,
		PreserveRecent:     1,
		Summarizer: func(messages []types.Message) (string, error) {
			summarized = append([]types.Message(nil), messages...)
			return "ORIGINAL-GOAL, CONSTRAINT, VERIFICATION, and PENDING-WORK preserved", nil
		},
	})

	if result.Skipped {
		t.Fatal("Compact() skipped oversized session")
	}
	if len(summarized) == 0 || !sessionMessagesContain(summarized, "ORIGINAL-GOAL") {
		t.Fatalf("summarizer received the wrong removed prefix: %#v", summarized)
	}
	if result.CompactedSession.Persona != session.Persona {
		t.Fatalf("compacted persona = %q, want %q", result.CompactedSession.Persona, session.Persona)
	}
	if result.CompactedSession.PlanMode != session.PlanMode {
		t.Fatalf("compacted plan mode = %v, want %v", result.CompactedSession.PlanMode, session.PlanMode)
	}
	if !sessionMessagesContain(result.CompactedSession.Messages, "ORIGINAL-GOAL") {
		t.Fatal("compacted messages lost the original goal")
	}

	path := filepath.Join(t.TempDir(), "compacted-session.json")
	if err := result.CompactedSession.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile() error = %v", err)
	}
	loaded, err := LoadSession(path)
	if err != nil {
		t.Fatalf("LoadSession() error = %v", err)
	}
	if loaded.Persona != session.Persona || loaded.PlanMode != session.PlanMode {
		t.Fatalf("persisted state = persona %q, plan %v; want persona %q, plan %v",
			loaded.Persona, loaded.PlanMode, session.Persona, session.PlanMode)
	}
}

func TestSessionTokenEstimateCountsEveryContentBlock(t *testing.T) {
	session := NewSession("test-model")
	session.Messages = []types.Message{{
		Role: types.RoleAssistant,
		Content: []types.ContentBlock{
			types.ThinkingBlock{Thinking: strings.Repeat("h", 400)},
			types.ToolUseBlock{ID: "call-1", Name: "read", Input: map[string]any{"path": strings.Repeat("p", 80)}},
			types.ToolResultBlock{ToolUseID: "call-1", Content: strings.Repeat("r", 120)},
		},
	}}

	if got := session.EstimateTokens(); got < 140 {
		t.Fatalf("EstimateTokens() = %d, want thinking, tool-use, and tool-result content counted", got)
	}
}

func sessionMessagesContain(messages []types.Message, want string) bool {
	for _, message := range messages {
		for _, block := range message.Content {
			switch value := block.(type) {
			case types.TextBlock:
				if strings.Contains(value.Text, want) {
					return true
				}
			case types.ToolResultBlock:
				if strings.Contains(value.Content, want) {
					return true
				}
			}
		}
	}
	return false
}
