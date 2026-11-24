package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// feedbackTestApp builds a handler-test app with the feedback lab set by
// pin and the data home isolated, so the file the command writes is the
// test's own. It returns the app and the feedback file path.
func feedbackTestApp(t *testing.T, pin bool) (*App, string) {
	t.Helper()
	dataHome := t.TempDir()
	t.Setenv("AGENT_HARNESS_DATA_HOME", dataHome)
	cfg := &config.LayeredConfig{
		Provider:   "ollama",
		LabsPinned: map[string]bool{config.LabFeedback: pin},
	}
	app := newHandlerTestApp(t, cfg, "test-model")
	app.initCommandsCore()
	return app, filepath.Join(dataHome, "feedback.jsonl")
}

// countingClient counts model requests. A /feedback command must never
// reach it.
type countingClient struct{ calls atomic.Int32 }

func (c *countingClient) Stream(ctx context.Context, req llm.Request) (<-chan types.LLMEvent, error) {
	c.calls.Add(1)
	ch := make(chan types.LLMEvent)
	close(ch)
	return ch, nil
}

func readFeedbackLines(t *testing.T, path string) [][]byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	trimmed := bytes.TrimSuffix(data, []byte("\n"))
	if len(trimmed) == 0 {
		return nil
	}
	return bytes.Split(trimmed, []byte("\n"))
}

func lastFeedbackRecord(t *testing.T, path string) map[string]any {
	t.Helper()
	lines := readFeedbackLines(t, path)
	if len(lines) == 0 {
		t.Fatalf("no feedback lines in %s", path)
	}
	var rec map[string]any
	if err := json.Unmarshal(lines[len(lines)-1], &rec); err != nil {
		t.Fatalf("feedback line is not JSON: %v", err)
	}
	return rec
}

func TestFeedbackCommandCategoriesAndNoCategory(t *testing.T) {
	cases := []struct {
		input string
		cat   string
		text  string
	}{
		{"/feedback bug crash on save", "bug", "crash on save"},
		{"/feedback annoying the spinner sticks", "annoying", "the spinner sticks"},
		{"/feedback missing a dark theme", "missing", "a dark theme"},
		{"/feedback other anything", "other", "anything"},
		{"/feedback no category here", "other", "no category here"},
	}
	for _, tc := range cases {
		app, path := feedbackTestApp(t, true)
		app.handleUserCommand(tc.input, tui.NewApp())
		rec := lastFeedbackRecord(t, path)
		if rec["category"] != tc.cat || rec["text"] != tc.text {
			t.Errorf("%q wrote category=%v text=%v, want %q / %q", tc.input, rec["category"], rec["text"], tc.cat, tc.text)
		}
	}
}

func TestFeedbackCommandEmptyTextShowsUsageAndWritesNothing(t *testing.T) {
	for _, input := range []string{"/feedback", "/feedback   ", "/feedback bug", "/feedback annoying   "} {
		app, path := feedbackTestApp(t, true)
		tuiApp := tui.NewApp()
		app.handleUserCommand(input, tuiApp)

		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%q created %s; want no file written (stat err %v)", input, path, err)
		}
		messages := chatMessages(t, tuiApp)
		if len(messages) != 1 || !strings.Contains(messages[0].content, "Usage: /feedback") {
			t.Errorf("%q chat = %+v, want one usage line", input, messages)
		}
	}
}

func TestFeedbackCommandAppendsOneRecordWithAgreedFieldsOnly(t *testing.T) {
	app, path := feedbackTestApp(t, true)
	tuiApp := tui.NewApp()

	app.handleUserCommand("/feedback bug crash on save", tuiApp)
	app.handleUserCommand("/feedback missing dark mode", tuiApp)

	lines := readFeedbackLines(t, path)
	if len(lines) != 2 {
		t.Fatalf("file has %d lines after two commands, want 2", len(lines))
	}
	first := map[string]any{}
	if err := json.Unmarshal(lines[0], &first); err != nil {
		t.Fatalf("first line is not JSON: %v", err)
	}
	keys := make([]string, 0, len(first))
	for k := range first {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := []string{"category", "harness_version", "model", "provider", "session_id", "text", "time"}
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Fatalf("record keys = %v, want exactly %v", keys, want)
	}
	if first["provider"] != "ollama" || first["model"] != "test-model" || first["session_id"] != app.session.ID {
		t.Fatalf("record identity = %v", first)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if mode := info.Mode().Perm(); mode != 0o600 {
			t.Fatalf("feedback file mode = %o, want 0600", mode)
		}
	}

	messages := chatMessages(t, tuiApp)
	if len(messages) == 0 || !strings.HasPrefix(messages[0].content, "Saved. Thanks. (") || !strings.Contains(messages[0].content, path) {
		t.Fatalf("acknowledgement = %+v, want Saved. Thanks. (%s)", messages, path)
	}
}

func TestFeedbackCommandStartsNoModelTurn(t *testing.T) {
	app, _ := feedbackTestApp(t, true)
	client := &countingClient{}
	app.client = client
	before := len(app.session.Messages)

	app.handleUserCommand("/feedback bug the model must not see this", tui.NewApp())

	if n := client.calls.Load(); n != 0 {
		t.Fatalf("model client called %d times, want 0", n)
	}
	if got := len(app.session.Messages); got != before {
		t.Fatalf("session gained %d messages, want none", got-before)
	}
}

func TestFeedbackHelpListsCommandOnlyWhenLabOn(t *testing.T) {
	off, _ := feedbackTestApp(t, false)
	offTUI := tui.NewApp()
	off.handleUserCommand("/help", offTUI)
	if msgs := chatMessages(t, offTUI); len(msgs) == 0 || strings.Contains(msgs[0].content, "/feedback") {
		t.Fatalf("/help with lab off lists /feedback: %+v", msgs)
	}

	on, _ := feedbackTestApp(t, true)
	onTUI := tui.NewApp()
	on.handleUserCommand("/help", onTUI)
	if msgs := chatMessages(t, onTUI); len(msgs) == 0 || !strings.Contains(msgs[0].content, "/feedback") {
		t.Fatalf("/help with lab on does not list /feedback: %+v", msgs)
	}
}

func TestFeedbackCommandWithLabOffAnswersLabsMessageAndWritesNothing(t *testing.T) {
	app, path := feedbackTestApp(t, false)
	tuiApp := tui.NewApp()

	app.handleUserCommand("/feedback bug crash", tuiApp)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("lab-off /feedback wrote %s", path)
	}
	messages := chatMessages(t, tuiApp)
	if len(messages) != 1 || messages[0].content != "Labs: enable feedback in Settings > Labs to use /feedback." {
		t.Fatalf("lab-off answer = %+v", messages)
	}
}
