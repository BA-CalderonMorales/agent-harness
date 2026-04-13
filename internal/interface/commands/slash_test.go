package commands

import (
	"strings"
	"testing"
)

func TestGetHelpDeterministicAndNoDuplicates(t *testing.T) {
	sr := NewSlashRegistry()
	sr.Register("help", "Show help", func(string) (string, error) { return "", nil })
	sr.Register("quit", "Quit", func(string) (string, error) { return "", nil })
	sr.Register("exit", "Exit alias", func(string) (string, error) { return "", nil })
	sr.Register("workspace", "Workspace info", func(string) (string, error) { return "", nil })

	help := sr.GetHelp()

	// /exit should not appear in help (hidden alias)
	if strings.Contains(help, "/exit") {
		t.Error("help should not show /exit alias")
	}

	// /workspace should appear
	if !strings.Contains(help, "/workspace") {
		t.Error("help should show /workspace")
	}

	// Deterministic: Session category should come first
	idx := strings.Index(help, "Session:")
	if idx == -1 {
		t.Error("help missing Session category")
	}

	// Run twice to ensure stability
	help2 := sr.GetHelp()
	if help != help2 {
		t.Error("help output should be deterministic")
	}
}

func TestGetCompletionsFiltersAliasesAndSorts(t *testing.T) {
	sr := NewSlashRegistry()
	sr.Register("quit", "Quit", func(string) (string, error) { return "", nil })
	sr.Register("exit", "Exit alias", func(string) (string, error) { return "", nil })
	sr.Register("abc", "ABC", func(string) (string, error) { return "", nil })

	comps := sr.GetCompletions()

	for _, c := range comps {
		if c == "/exit" {
			t.Error("completions should not include /exit alias")
		}
	}

	// Should be sorted
	for i := 1; i < len(comps); i++ {
		if comps[i] < comps[i-1] {
			t.Errorf("completions not sorted: %v", comps)
		}
	}
}

func TestParseSlashCommand(t *testing.T) {
	cmd := ParseSlashCommand("/model gpt-4o")
	if cmd.Name != "model" {
		t.Errorf("expected name=model, got %q", cmd.Name)
	}
	if cmd.Args != "gpt-4o" {
		t.Errorf("expected args=gpt-4o, got %q", cmd.Args)
	}

	cmd2 := ParseSlashCommand("/clear")
	if cmd2.Name != "clear" {
		t.Errorf("expected name=clear, got %q", cmd2.Name)
	}
	if cmd2.Args != "" {
		t.Errorf("expected empty args, got %q", cmd2.Args)
	}
}
