package commands

import (
	"errors"
	"strings"
	"testing"
)

func TestParseSlashCommand(t *testing.T) {
	tests := []struct {
		input    string
		wantName string
		wantArgs string
	}{
		{"/model gpt-4o", "model", "gpt-4o"},
		{"/clear", "clear", ""},
		{"/clear   --confirm", "clear", "--confirm"},
		{"/model  claude-3-5-sonnet  with spaces", "model", "claude-3-5-sonnet  with spaces"},
		{"/reset --confirm", "reset", "--confirm"},
		{"/session load abc-123", "session", "load abc-123"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cmd := ParseSlashCommand(tt.input)
			if cmd.Name != tt.wantName {
				t.Errorf("expected name=%q, got %q", tt.wantName, cmd.Name)
			}
			if cmd.Args != tt.wantArgs {
				t.Errorf("expected args=%q, got %q", tt.wantArgs, cmd.Args)
			}
		})
	}
}

func TestHandle(t *testing.T) {
	sr := NewSlashRegistry()
	sr.Register("help", "Show help", func(string) (string, error) { return "help text", nil })
	sr.Register("quit", "Quit", func(string) (string, error) { return "__QUIT__", nil })
	sr.Register("compact", "Compact", func(string) (string, error) { return "compacted", nil })

	tests := []struct {
		name        string
		input       string
		wantResult  string
		wantHandled bool
		wantErr     bool
	}{
		{"non-slash", "hello", "", false, false},
		{"exact match", "/help", "help text", true, false},
		{"unknown command", "/foo", "Unknown command: /foo", true, false},
		{"similar command prefix", "/comp", "Unknown command: /comp\nDid you mean: /compact?", true, false},
		{"similar command substring", "/quitty", "Unknown command: /quitty\nDid you mean: /quit?", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, handled, err := sr.Handle(tt.input)
			if handled != tt.wantHandled {
				t.Errorf("expected handled=%v, got %v", tt.wantHandled, handled)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("expected err=%v, got %v", tt.wantErr, err)
			}
			if handled && !strings.Contains(result, tt.wantResult) {
				t.Errorf("expected result to contain %q, got %q", tt.wantResult, result)
			}
		})
	}
}

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

func TestHelpHandler(t *testing.T) {
	sr := NewSlashRegistry()
	sr.Register("help", "Show available commands", HelpHandler(sr))
	sr.Register("quit", "Exit the application", QuitHandler())

	h := HelpHandler(sr)

	// No args
	result, err := h("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Available commands:") {
		t.Errorf("expected help text, got %q", result)
	}

	// Specific command
	result, err = h("quit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "/quit - Exit the application" {
		t.Errorf("expected specific help, got %q", result)
	}

	// Unknown command
	result, err = h("foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Unknown command: /foo" {
		t.Errorf("expected unknown command msg, got %q", result)
	}
}

func TestStatusHandler(t *testing.T) {
	h := StatusHandler(func() string { return "session: active\nmodel: gpt-4o" })
	result, err := h("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "session: active\nmodel: gpt-4o" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestClearHandler(t *testing.T) {
	var cleared bool
	var chatCleared bool

	h := ClearHandler(
		func() error { cleared = true; return nil },
		func(msg string) { chatCleared = true },
	)

	result, err := h("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// When clearChatFn is provided, result is empty to avoid double-adding the message
	if result != "" {
		t.Errorf("expected empty result when clearChatFn provided, got %q", result)
	}
	if !cleared {
		t.Error("expected clearFn to be called")
	}
	if !chatCleared {
		t.Error("expected clearChatFn to be called")
	}

	// Error case
	failH := ClearHandler(func() error { return errors.New("disk full") }, nil)
	_, err = failH("")
	if err == nil || err.Error() != "disk full" {
		t.Errorf("expected disk full error, got %v", err)
	}
}

func TestCompactHandler(t *testing.T) {
	h := CompactHandler(func() (string, error) { return "Compacted: removed 5 messages", nil })
	result, err := h("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Compacted: removed 5 messages" {
		t.Errorf("unexpected result: %q", result)
	}

	failH := CompactHandler(func() (string, error) { return "", errors.New("compaction failed") })
	_, err = failH("")
	if err == nil || err.Error() != "compaction failed" {
		t.Errorf("expected compaction failed error, got %v", err)
	}
}

func TestCostHandler(t *testing.T) {
	h := CostHandler(func() string { return "Cost: $0.42" })
	result, err := h("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Cost: $0.42" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestCurrentModelHandler(t *testing.T) {
	h := CurrentModelHandler(func() string { return "gpt-4o" })
	result, err := h("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Current model: gpt-4o" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestModelHandler(t *testing.T) {
	var current string = "gpt-4o"
	h := ModelHandler(
		func() string { return current },
		func(m string) error { current = m; return nil },
		func() []string { return []string{"gpt-4o", "claude-3-5-sonnet"} },
	)

	// No args - list models
	result, err := h("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "gpt-4o") {
		t.Errorf("expected model list to contain gpt-4o, got %q", result)
	}
	if !strings.Contains(result, "● gpt-4o") {
		t.Errorf("expected current model marker, got %q", result)
	}

	// With args - switch model
	result, err = h("claude-3-5-sonnet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Model updated") {
		t.Errorf("expected model updated message, got %q", result)
	}
	if current != "claude-3-5-sonnet" {
		t.Errorf("expected model to change, got %q", current)
	}

	// Error case
	failH := ModelHandler(
		func() string { return "gpt-4o" },
		func(m string) error { return errors.New("invalid model") },
		func() []string { return nil },
	)
	_, err = failH("bad-model")
	if err == nil || err.Error() != "invalid model" {
		t.Errorf("expected invalid model error, got %v", err)
	}
}

func TestPermissionsHandler(t *testing.T) {
	var current string = "read-only"
	h := PermissionsHandler(
		func() string { return current },
		func(m string) error { current = m; return nil },
		func() string { return "Mode: read-only" },
	)

	// No args
	result, err := h("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Mode: read-only" {
		t.Errorf("unexpected result: %q", result)
	}

	// With args
	result, err = h("workspace-write")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Permissions updated") {
		t.Errorf("expected permissions updated message, got %q", result)
	}
	if current != "workspace-write" {
		t.Errorf("expected mode to change, got %q", current)
	}

	// Error case
	failH := PermissionsHandler(
		func() string { return "read-only" },
		func(m string) error { return errors.New("invalid mode") },
		func() string { return "" },
	)
	_, err = failH("bad-mode")
	if err == nil || err.Error() != "invalid mode" {
		t.Errorf("expected invalid mode error, got %v", err)
	}
}

func TestConfigHandler(t *testing.T) {
	h := ConfigHandler(func() string { return "provider: openrouter\nmodel: gpt-4o" })
	result, err := h("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "provider: openrouter\nmodel: gpt-4o" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestExportHandler(t *testing.T) {
	h := ExportHandler(func(path string) (string, error) {
		if path == "" {
			path = "session-default.json"
		}
		return path, nil
	})

	result, err := h("my-export.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "my-export.json") {
		t.Errorf("expected result to contain my-export.json, got %q", result)
	}

	result, err = h("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "session-default.json") {
		t.Errorf("expected result to contain session-default.json, got %q", result)
	}

	failH := ExportHandler(func(path string) (string, error) { return "", errors.New("write failed") })
	_, err = failH("/bad/path")
	if err == nil || err.Error() != "write failed" {
		t.Errorf("expected write failed error, got %v", err)
	}
}

func TestDiffHandler(t *testing.T) {
	// Empty diff
	hEmpty := DiffHandler(func() string { return "" })
	result, err := hEmpty("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "No changes detected in workspace." {
		t.Errorf("unexpected result: %q", result)
	}

	// Non-empty diff
	hDiff := DiffHandler(func() string { return "+added line" })
	result, err = hDiff("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "+added line" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestVersionHandler(t *testing.T) {
	h := VersionHandler("0.1.4", "Built: 2024-01-01")
	result, err := h("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "agent-harness 0.1.4\nBuilt: 2024-01-01"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}

	hNoBuild := VersionHandler("0.1.4", "")
	result, err = hNoBuild("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "agent-harness 0.1.4" {
		t.Errorf("expected version only, got %q", result)
	}
}

func TestMemoryHandler(t *testing.T) {
	h := MemoryHandler(func() string { return "Memory: 42 tokens" })
	result, err := h("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Memory: 42 tokens" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestAgentsHandler(t *testing.T) {
	h := AgentsHandler(func(args string) string {
		if args == "" {
			return "All agents"
		}
		return "Agent: " + args
	})

	result, err := h("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "All agents" {
		t.Errorf("unexpected result: %q", result)
	}

	result, err = h("custom")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Agent: custom" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestSkillsHandler(t *testing.T) {
	h := SkillsHandler(func(args string) string { return "Skills: coding, writing" })
	result, err := h("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Skills: coding, writing" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestSessionHandler(t *testing.T) {
	var loadedID string
	h := SessionHandler(
		func() string { return "session-1\nsession-2" },
		func(id string) error { loadedID = id; return nil },
	)

	// List
	result, err := h("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "session-1\nsession-2" {
		t.Errorf("unexpected result: %q", result)
	}

	result, err = h("list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "session-1\nsession-2" {
		t.Errorf("unexpected result: %q", result)
	}

	// Load
	result, err = h("load abc-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loadedID != "abc-123" {
		t.Errorf("expected loadedID=abc-123, got %q", loadedID)
	}
	if result != "Session loaded: abc-123" {
		t.Errorf("unexpected result: %q", result)
	}

	// Invalid usage
	result, err = h("delete")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Usage: /session [list|load <id>]" {
		t.Errorf("unexpected result: %q", result)
	}

	// Load error
	failH := SessionHandler(
		func() string { return "" },
		func(id string) error { return errors.New("session not found") },
	)
	_, err = failH("load missing")
	if err == nil || err.Error() != "session not found" {
		t.Errorf("expected session not found error, got %v", err)
	}
}

func TestResetHandler(t *testing.T) {
	var resetCalled bool
	h := ResetHandler(func() error { resetCalled = true; return nil })

	// Without confirmation
	result, err := h("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "WARNING") {
		t.Errorf("expected warning message, got %q", result)
	}
	if resetCalled {
		t.Error("reset should not be called without confirmation")
	}

	// With --confirm
	result, err = h("--confirm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "__RESET__" {
		t.Errorf("expected __RESET__, got %q", result)
	}
	if !resetCalled {
		t.Error("reset should be called with confirmation")
	}

	// With -y
	resetCalled = false
	result, err = h("-y")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "__RESET__" {
		t.Errorf("expected __RESET__, got %q", result)
	}
	if !resetCalled {
		t.Error("reset should be called with -y")
	}

	// Error case
	failH := ResetHandler(func() error { return errors.New("reset failed") })
	_, err = failH("--confirm")
	if err == nil || err.Error() != "reset failed" {
		t.Errorf("expected reset failed error, got %v", err)
	}
}

func TestQuitHandler(t *testing.T) {
	h := QuitHandler()
	result, err := h("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "__QUIT__" {
		t.Errorf("expected __QUIT__, got %q", result)
	}
	if !IsQuit(result) {
		t.Error("IsQuit should return true for __QUIT__")
	}
	if IsQuit("something else") {
		t.Error("IsQuit should return false for non-quit results")
	}
}

func TestWorkspaceHandler(t *testing.T) {
	h := WorkspaceHandler(func() string { return "Workspace: /home/user/projects" })
	result, err := h("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Workspace: /home/user/projects" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestFindSimilar(t *testing.T) {
	sr := NewSlashRegistry()
	sr.Register("help", "Show help", func(string) (string, error) { return "", nil })
	sr.Register("hello", "Say hello", func(string) (string, error) { return "", nil })
	sr.Register("quit", "Quit", func(string) (string, error) { return "", nil })

	// Prefix match
	result, handled, _ := sr.Handle("/hel")
	if !handled {
		t.Fatal("expected handled")
	}
	if !strings.Contains(result, "/help") && !strings.Contains(result, "/hello") {
		t.Errorf("expected suggestions for hel, got %q", result)
	}

	// Substring match (command name contains query)
	result, handled, _ = sr.Handle("/qu")
	if !handled {
		t.Fatal("expected handled")
	}
	if !strings.Contains(result, "/quit") {
		t.Errorf("expected /quit suggestion, got %q", result)
	}
}
