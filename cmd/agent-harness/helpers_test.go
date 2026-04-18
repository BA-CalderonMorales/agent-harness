package main

import (
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/state"
	"github.com/BA-CalderonMorales/agent-harness/internal/skills"
)

// =============================================================================
// formatSessionList
// =============================================================================

func TestFormatSessionList(t *testing.T) {
	t.Run("empty sessions", func(t *testing.T) {
		result := formatSessionList([]state.SessionMetadata{}, "abc")
		if result != "No saved sessions." {
			t.Errorf("expected 'No saved sessions.', got %q", result)
		}
	})

	t.Run("single session without active marker", func(t *testing.T) {
		sessions := []state.SessionMetadata{
			{ID: "session-123", MessageCount: 5, Turns: 2},
		}
		result := formatSessionList(sessions, "other")
		if !strings.Contains(result, "session-") {
			t.Errorf("expected truncated session ID, got %q", result)
		}
		if strings.Contains(result, "(active)") {
			t.Error("should not mark non-current as active")
		}
	})

	t.Run("marks current session active", func(t *testing.T) {
		sessions := []state.SessionMetadata{
			{ID: "session-abc", MessageCount: 3, Turns: 1},
		}
		result := formatSessionList(sessions, "session-abc")
		if !strings.Contains(result, "(active)") {
			t.Errorf("expected active marker, got %q", result)
		}
	})

	t.Run("multiple sessions", func(t *testing.T) {
		sessions := []state.SessionMetadata{
			{ID: "session-aaa", MessageCount: 1, Turns: 0},
			{ID: "session-bbb", MessageCount: 10, Turns: 5},
		}
		result := formatSessionList(sessions, "session-bbb")
		lines := strings.Split(result, "\n")
		if len(lines) != 3 {
			t.Errorf("expected 3 lines (header + 2 sessions), got %d: %q", len(lines), result)
		}
		if !strings.Contains(result, "session-") {
			t.Error("missing session truncated ID")
		}
		if !strings.Contains(result, "1 message") {
			t.Error("missing session-aaa message count")
		}
		if !strings.Contains(result, "10 messages") {
			t.Error("missing session-bbb message count")
		}
	})
}

// =============================================================================
// formatSkillsList / formatSkillDetail / firstLine
// =============================================================================

func TestFormatSkillsList(t *testing.T) {
	t.Run("empty skills", func(t *testing.T) {
		result := formatSkillsList([]skills.Skill{})
		if result != "No skills available." {
			t.Errorf("expected 'No skills available.', got %q", result)
		}
	})

	t.Run("single skill", func(t *testing.T) {
		skillsList := []skills.Skill{
			{Name: "coding", Description: "Write code efficiently", Content: "line1\nline2\nline3"},
		}
		result := formatSkillsList(skillsList)
		if !strings.Contains(result, "coding") {
			t.Errorf("expected skill name, got %q", result)
		}
		if !strings.Contains(result, "Write code efficiently") {
			t.Errorf("expected description, got %q", result)
		}
		if !strings.Contains(result, "(3 lines)") {
			t.Errorf("expected line count, got %q", result)
		}
	})

	t.Run("long description truncation", func(t *testing.T) {
		skillsList := []skills.Skill{
			{Name: "doc", Description: strings.Repeat("a", 80), Content: "x"},
		}
		result := formatSkillsList(skillsList)
		if !strings.Contains(result, "...") {
			t.Errorf("expected truncated description with ..., got %q", result)
		}
	})
}

func TestFormatSkillDetail(t *testing.T) {
	t.Run("full skill detail", func(t *testing.T) {
		sk := skills.Skill{Name: "test", Path: "/tmp/test.md", Content: "content here"}
		result := formatSkillDetail(sk)
		if !strings.Contains(result, "Skill: test") {
			t.Errorf("expected skill name, got %q", result)
		}
		if !strings.Contains(result, "Path:  /tmp/test.md") {
			t.Errorf("expected path, got %q", result)
		}
		if !strings.Contains(result, "content here") {
			t.Errorf("expected content, got %q", result)
		}
	})
}

func TestFirstLine(t *testing.T) {
	t.Run("single line", func(t *testing.T) {
		result := firstLine("hello world")
		if result != "hello world" {
			t.Errorf("expected 'hello world', got %q", result)
		}
	})

	t.Run("multi line", func(t *testing.T) {
		result := firstLine("first\nsecond\nthird")
		if result != "first" {
			t.Errorf("expected 'first', got %q", result)
		}
	})

	t.Run("leading whitespace", func(t *testing.T) {
		result := firstLine("  \n  trimmed  ")
		if result != "trimmed" {
			t.Errorf("expected 'trimmed', got %q", result)
		}
	})

	t.Run("empty string", func(t *testing.T) {
		result := firstLine("")
		if result != "" {
			t.Errorf("expected empty, got %q", result)
		}
	})
}

// =============================================================================
// Model resolution helpers
// =============================================================================

func TestGetDefaultModel(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{"openai", "gpt-4o"},
		{"anthropic", "claude-3-5-sonnet-20241022"},
		{"ollama", "gemma4:2b"},
		{"openrouter", "nvidia/nemotron-3-super-120b-a12b:free"},
		{"", "nvidia/nemotron-3-super-120b-a12b:free"},
		{"unknown", "nvidia/nemotron-3-super-120b-a12b:free"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := getDefaultModel(tt.provider)
			if got != tt.want {
				t.Errorf("getDefaultModel(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

func TestResolveModelInput(t *testing.T) {
	tests := []struct {
		input    string
		provider string
		want     string
	}{
		{"", "openrouter", ""},
		{"1", "openai", "gpt-4o"},
		{"2", "openai", "gpt-4o-mini"},
		{"3", "openai", "gpt-4-turbo"},
		{"1", "anthropic", "claude-3-5-sonnet-20241022"},
		{"2", "anthropic", "claude-3-opus-20240229"},
		{"1", "openrouter", "nvidia/nemotron-3-super-120b-a12b:free"},
		{"2", "openrouter", "anthropic/claude-3.5-sonnet"},
		{"3", "openrouter", "openai/gpt-4o"},
		{"custom-model", "openrouter", "custom-model"},
	}

	for _, tt := range tests {
		t.Run(tt.input+"_"+tt.provider, func(t *testing.T) {
			got := resolveModelInput(tt.input, tt.provider)
			if got != tt.want {
				t.Errorf("resolveModelInput(%q, %q) = %q, want %q", tt.input, tt.provider, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Tool classification helpers
// =============================================================================

func TestIsReadOnlyTool(t *testing.T) {
	readOnly := []string{"read", "glob", "grep", "search", "web_fetch", "web_search"}
	for _, name := range readOnly {
		t.Run(name, func(t *testing.T) {
			if !isReadOnlyTool(name) {
				t.Errorf("expected %s to be read-only", name)
			}
		})
	}

	notReadOnly := []string{"bash", "write", "edit", "delete"}
	for _, name := range notReadOnly {
		t.Run("not_"+name, func(t *testing.T) {
			if isReadOnlyTool(name) {
				t.Errorf("expected %s to NOT be read-only", name)
			}
		})
	}
}

func TestIsDangerousTool(t *testing.T) {
	dangerous := []string{"bash", "write", "edit"}
	for _, name := range dangerous {
		t.Run(name, func(t *testing.T) {
			if !isDangerousTool(name) {
				t.Errorf("expected %s to be dangerous", name)
			}
		})
	}

	notDangerous := []string{"read", "glob", "grep", "search"}
	for _, name := range notDangerous {
		t.Run("not_"+name, func(t *testing.T) {
			if isDangerousTool(name) {
				t.Errorf("expected %s to NOT be dangerous", name)
			}
		})
	}
}

func TestStringSliceContains(t *testing.T) {
	slice := []string{"a", "b", "c"}

	if !stringSliceContains(slice, "a") {
		t.Error("expected 'a' to be found")
	}
	if !stringSliceContains(slice, "c") {
		t.Error("expected 'c' to be found")
	}
	if stringSliceContains(slice, "d") {
		t.Error("expected 'd' to NOT be found")
	}
	if stringSliceContains([]string{}, "a") {
		t.Error("expected empty slice to not contain anything")
	}
}
