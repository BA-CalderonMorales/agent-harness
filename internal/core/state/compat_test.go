package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/testharness/testenv"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// Upgrade compatibility: the fixtures in testdata/compat are session files
// that released and development builds actually wrote, sanitized by
// scripts/verify/sanitize-session.py. Every build must keep loading,
// resuming, and appending to them, because they are what a user's machine
// holds when they upgrade.

type compatFixture struct {
	name        string
	version     int
	messages    int
	toolUses    int
	toolResults int
	toolErrors  int
}

var compatFixtures = []compatFixture{
	// Header and a meta update only: a session opened and left empty.
	{name: "v1-header-meta", version: 1},
	// Tool calls, results, and an error result, plus a tool call that never
	// received a result.
	{name: "v1-tools-errors", version: 1, messages: 10, toolUses: 5, toolResults: 4, toolErrors: 1},
	// Header version 2 with turns, and a tool result whose call is not in
	// the transcript.
	{name: "v2-turns-orphan", version: 2, messages: 14, toolUses: 6, toolResults: 7, toolErrors: 1},
	// Text messages and meta updates from the current build.
	{name: "v1-text-latest", version: 1, messages: 4},
}

func compatFixturePath(name string) string {
	return filepath.Join("testdata", "compat", name+".jsonl")
}

func countBlocks(messages []types.Message) (uses, results, errors int) {
	for _, msg := range messages {
		for _, block := range msg.Content {
			switch b := block.(type) {
			case types.ToolUseBlock:
				uses++
			case types.ToolResultBlock:
				results++
				if b.IsError {
					errors++
				}
			}
		}
	}
	return uses, results, errors
}

// installCompatFixture copies a fixture into an isolated home as the only
// saved session of a project, the way an upgraded user's data looks.
func installCompatFixture(t *testing.T, name string) (*SessionManager, string) {
	t.Helper()
	testenv.SetHome(t, t.TempDir())
	sessionsDir := t.TempDir()
	t.Setenv("AGENT_HARNESS_SESSION_DIR", sessionsDir)

	root := filepath.Join(t.TempDir(), "project")
	sm, err := NewSessionManagerForProject(root)
	if err != nil {
		t.Fatalf("NewSessionManagerForProject: %v", err)
	}
	data, err := os.ReadFile(compatFixturePath(name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	session, err := loadSessionFile(compatFixturePath(name))
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	dir := sm.projectSessionsDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Saves locate a session's file by its start time and ID, and every
	// real file is named that way, so the fixture is installed under the
	// same name.
	path := filepath.Join(dir, sessionFileName(session))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("install fixture: %v", err)
	}
	return sm, path
}

// rawFixture reads a fixture with plain JSON decoding, independent of the
// loader under test: the messages exactly as the file holds them, and the
// turns a resume must restore (the header's, then each meta update's).
func rawFixture(t *testing.T, name string) (messages []map[string]any, turns int) {
	t.Helper()
	data, err := os.ReadFile(compatFixturePath(name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("fixture line is not JSON: %v", err)
		}
		switch record["type"] {
		case "header":
			if n, ok := record["turns"].(float64); ok {
				turns = int(n)
			}
		case "meta":
			meta, _ := record["meta"].(map[string]any)
			n, _ := meta["turns"].(float64)
			turns = int(n)
		case "message":
			messages = append(messages, record["message"].(map[string]any))
		}
	}
	return messages, turns
}

func rawString(fields map[string]any, key string) string {
	value, _ := fields[key].(string)
	return value
}

// assertMatchesRaw compares loaded messages field by field with the raw
// file, so a decoder that drops text, tool IDs, or links cannot pass.
func assertMatchesRaw(t *testing.T, loaded []types.Message, raw []map[string]any) {
	t.Helper()
	if len(loaded) != len(raw) {
		t.Fatalf("loaded %d messages, the file holds %d", len(loaded), len(raw))
	}
	for i, msg := range loaded {
		want := raw[i]
		if msg.UUID != rawString(want, "uuid") || string(msg.Role) != rawString(want, "role") ||
			msg.StopReason != rawString(want, "stop_reason") || msg.Model != rawString(want, "model") {
			t.Fatalf("message %d header fields = %q/%q/%q/%q, file has %q/%q/%q/%q", i,
				msg.UUID, msg.Role, msg.StopReason, msg.Model,
				rawString(want, "uuid"), rawString(want, "role"), rawString(want, "stop_reason"), rawString(want, "model"))
		}
		if stamp, err := time.Parse(time.RFC3339Nano, rawString(want, "timestamp")); err != nil || !stamp.Equal(msg.Timestamp) {
			t.Fatalf("message %d timestamp = %v, file has %q", i, msg.Timestamp, rawString(want, "timestamp"))
		}
		blocks, _ := want["content"].([]any)
		if len(msg.Content) != len(blocks) {
			t.Fatalf("message %d has %d blocks, the file holds %d", i, len(msg.Content), len(blocks))
		}
		for j, block := range msg.Content {
			fields := blocks[j].(map[string]any)
			switch b := block.(type) {
			case types.TextBlock:
				if b.Text != rawString(fields, "text") {
					t.Fatalf("message %d block %d text differs from the file", i, j)
				}
			case types.ToolUseBlock:
				input, _ := json.Marshal(fields["input"])
				got, _ := json.Marshal(b.Input)
				if b.ID != rawString(fields, "id") || b.Name != rawString(fields, "name") || string(got) != string(input) {
					t.Fatalf("message %d block %d tool call = %q %q %s, file has %q %q %s", i, j,
						b.ID, b.Name, got, rawString(fields, "id"), rawString(fields, "name"), input)
				}
			case types.ToolResultBlock:
				isError, _ := fields["is_error"].(bool)
				if b.ToolUseID != rawString(fields, "tool_use_id") || b.Content != rawString(fields, "content") || b.IsError != isError {
					t.Fatalf("message %d block %d tool result = %q error=%v, file has %q error=%v", i, j,
						b.ToolUseID, b.IsError, rawString(fields, "tool_use_id"), isError)
				}
			default:
				t.Fatalf("message %d block %d decoded as %T, which the file does not hold", i, j, block)
			}
		}
	}
}

func messagesJSON(t *testing.T, messages []types.Message) []string {
	t.Helper()
	out := make([]string, len(messages))
	for i, msg := range messages {
		data, err := json.Marshal(msg)
		if err != nil {
			t.Fatalf("marshal message %d: %v", i, err)
		}
		out[i] = string(data)
	}
	return out
}

func TestCompatFixturesLoadWithEveryBlockKind(t *testing.T) {
	for _, fixture := range compatFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			session, err := loadSessionFile(compatFixturePath(fixture.name))
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if session.Version != fixture.version {
				t.Errorf("version = %d, want %d", session.Version, fixture.version)
			}
			if len(session.Messages) != fixture.messages {
				t.Fatalf("messages = %d, want %d", len(session.Messages), fixture.messages)
			}
			uses, results, errors := countBlocks(session.Messages)
			if uses != fixture.toolUses || results != fixture.toolResults || errors != fixture.toolErrors {
				t.Errorf("tool uses/results/errors = %d/%d/%d, want %d/%d/%d",
					uses, results, errors, fixture.toolUses, fixture.toolResults, fixture.toolErrors)
			}
			if session.Model == "" || session.Persona == "" {
				t.Errorf("model %q / persona %q lost on load", session.Model, session.Persona)
			}
			raw, turns := rawFixture(t, fixture.name)
			assertMatchesRaw(t, session.Messages, raw)
			if session.Turns != turns {
				t.Errorf("turns = %d, the file records %d", session.Turns, turns)
			}
		})
	}
}

func TestCompatFixtureMessagesSurviveAJSONRoundTrip(t *testing.T) {
	for _, fixture := range compatFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			session, err := loadSessionFile(compatFixturePath(fixture.name))
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			for i, msg := range session.Messages {
				data, err := json.Marshal(msg)
				if err != nil {
					t.Fatalf("marshal message %d: %v", i, err)
				}
				var back types.Message
				if err := json.Unmarshal(data, &back); err != nil {
					t.Fatalf("unmarshal message %d: %v", i, err)
				}
				if !reflect.DeepEqual(msg, back) {
					t.Fatalf("message %d changed in a round trip:\nbefore %#v\nafter  %#v", i, msg, back)
				}
			}
		})
	}
}

func TestCompatFixturesResumeAndAcceptNewMessages(t *testing.T) {
	for _, fixture := range compatFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			sm, path := installCompatFixture(t, fixture.name)
			original, err := loadSessionFile(path)
			if err != nil {
				t.Fatalf("load installed fixture: %v", err)
			}

			resumed, err := sm.ResumeLatestSessionWithError()
			if err != nil || resumed == nil {
				t.Fatalf("resume = (%v, %v), want the installed session", resumed, err)
			}
			if resumed.ID != original.ID {
				t.Fatalf("resumed %q, want %q", resumed.ID, original.ID)
			}
			before := messagesJSON(t, original.Messages)

			added := types.Message{
				UUID:      "msg-after-upgrade",
				Role:      types.RoleUser,
				Content:   []types.ContentBlock{types.TextBlock{Text: "a message written by the upgraded build"}},
				Timestamp: time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC),
			}
			resumed.Messages = append(resumed.Messages, added)
			if _, err := sm.SaveCurrent(); err != nil {
				t.Fatalf("save after resume: %v", err)
			}

			reloaded, err := loadSessionFile(path)
			if err != nil {
				t.Fatalf("reload after append: %v", err)
			}
			after := messagesJSON(t, reloaded.Messages)
			if len(after) != len(before)+1 {
				t.Fatalf("messages after append = %d, want %d", len(after), len(before)+1)
			}
			if !reflect.DeepEqual(after[:len(before)], before) {
				t.Fatal("appending changed the messages the earlier build wrote")
			}
			raw, _ := rawFixture(t, fixture.name)
			assertMatchesRaw(t, reloaded.Messages[:len(raw)], raw)
			if want := messagesJSON(t, []types.Message{added})[0]; after[len(after)-1] != want {
				t.Fatalf("the appended message reloaded as %s, want %s", after[len(after)-1], want)
			}
			if reloaded.Version != fixture.version || reloaded.Model != original.Model {
				t.Errorf("header changed: version %d model %q, want %d %q",
					reloaded.Version, reloaded.Model, fixture.version, original.Model)
			}
		})
	}
}

// Compaction shrinks the transcript below what is on disk, which makes
// SaveCurrent rewrite the whole file in the current format.
func TestCompatFixturesSurviveAFullRewrite(t *testing.T) {
	for _, fixture := range compatFixtures {
		if fixture.messages < 2 {
			continue
		}
		t.Run(fixture.name, func(t *testing.T) {
			sm, path := installCompatFixture(t, fixture.name)
			resumed, err := sm.ResumeLatestSessionWithError()
			if err != nil || resumed == nil {
				t.Fatalf("resume = (%v, %v)", resumed, err)
			}
			if _, err := sm.SaveCurrent(); err != nil {
				t.Fatalf("first save: %v", err)
			}
			kept := append([]types.Message(nil), resumed.Messages[len(resumed.Messages)/2:]...)
			resumed.Messages = kept
			if _, err := sm.SaveCurrent(); err != nil {
				t.Fatalf("save after shrinking: %v", err)
			}

			reloaded, err := loadSessionFile(path)
			if err != nil {
				t.Fatalf("reload after rewrite: %v", err)
			}
			if !reflect.DeepEqual(messagesJSON(t, reloaded.Messages), messagesJSON(t, kept)) {
				t.Fatal("the rewritten file does not hold exactly the kept messages")
			}
			raw, _ := rawFixture(t, fixture.name)
			assertMatchesRaw(t, reloaded.Messages, raw[len(raw)-len(kept):])
		})
	}
}
