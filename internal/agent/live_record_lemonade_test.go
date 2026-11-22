package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/record"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

const liveResumeQuestion = "Resume: state the goal, one constraint, and what is still pending."

// liveClient counts requests by kind, so a run reports how many model
// requests compaction itself cost.
type liveClient struct {
	llm.Client
	summaries atomic.Int32
	mains     atomic.Int32
	mu        sync.Mutex
	lastMain  llm.Request
}

func (c *liveClient) Stream(ctx context.Context, req llm.Request) (<-chan types.LLMEvent, error) {
	if strings.Contains(req.SystemPrompt, "context summarizer") {
		c.summaries.Add(1)
	} else {
		c.mains.Add(1)
		c.mu.Lock()
		c.lastMain = req
		c.mu.Unlock()
	}
	return c.Client.Stream(ctx, req)
}

// liveSession is a synthetic long session, about three 4096-token windows:
// goal, constraint, tool calls on named files, one failing test command and
// a todo plan with two pending items. Nothing in it is real project data.
func liveSession() []types.Message {
	msgs := []types.Message{
		userText("Migrate settings persistence to one write path. Never push directly to main."),
		assistantToolCall("l1", "todo_write", map[string]any{"todos": []any{
			map[string]any{"text": "unify the settings writers", "status": "done"},
			map[string]any{"text": "add the regression test for concurrent saves", "status": "pending"},
			map[string]any{"text": "update the changelog entry", "status": "pending"},
		}}),
		toolResult("l1", "ok", false),
	}
	files := []string{"internal/core/state/settings_store.go", "internal/core/state/settings_writer.go", "cmd/agent-harness/settings_commit.go", "internal/core/config/save.go"}
	for i, f := range files {
		id := fmt.Sprintf("lr%d", i)
		msgs = append(msgs, assistantToolCall(id, "read", map[string]any{"file_path": f}), toolResult(id, "package state // contents elided", false))
	}
	for i, f := range files[:3] {
		id := fmt.Sprintf("le%d", i)
		msgs = append(msgs, assistantToolCall(id, "edit", map[string]any{"file_path": f}), toolResult(id, "edited", false))
	}
	msgs = append(msgs, assistantToolCall("lb", "bash", map[string]any{"command": "go test ./internal/core/state/..."}),
		toolResult("lb", "FAIL TestSettingsConcurrentSave: second writer overwrote the first", true))
	for i := 0; len(msgs) < 400 && estimateTokens(msgs) < 3*4096; i++ {
		msgs = append(msgs, userText("keep going"),
			assistantText(fmt.Sprintf("Step %d: reviewed the call sites of the old save helpers and compared them with the new writer. %s", i, strings.Repeat("Details of the review follow. ", 12))))
	}
	return append(msgs, userText(liveResumeQuestion))
}

type liveScore struct {
	pass     bool
	notes    []string
	answer   string
	latency  time.Duration
	compacts int32
	failure  string
	// seen is the conversation text of the final main request, minus the
	// padded system prompt, so a failure can be traced to what the model saw.
	seen string
}

var (
	// mainWord is "main" as a word, never inside one ("domain").
	mainWord = regexp.MustCompile(`\bmain\b`)
	// negatedPush is a negation shortly before a form of "push": "never
	// push", "no direct pushes", "must not push". A negation after the push
	// ("pushed to main, no conflicts") does not count.
	negatedPush = regexp.MustCompile(`\b(never|no|not|don't|avoid|without|forbid\w*|prohibit\w*)\b[^.!?]{0,30}?\bpush`)
)

// statesNoPushToMain reports whether one sentence of a lowercased answer
// forbids pushing to main: it names main as a word and carries a negation
// before the push. The bare word "main" ("the main goal") is not a constraint.
func statesNoPushToMain(a string) bool {
	for _, sentence := range strings.FieldsFunc(a, func(r rune) bool { return r == '.' || r == '!' || r == '?' || r == '\n' }) {
		if mainWord.MatchString(sentence) && negatedPush.MatchString(sentence) {
			return true
		}
	}
	return false
}

func liveScoreAnswer(answer string) (bool, []string) {
	a := strings.ToLower(answer)
	checks := []struct {
		name string
		ok   bool
	}{
		{"goal", strings.Contains(a, "settings") && (strings.Contains(a, "write path") || strings.Contains(a, "persistence"))},
		{"constraint", statesNoPushToMain(a)},
		{"todo-test", strings.Contains(a, "regression test") || strings.Contains(a, "concurrent")},
		{"todo-changelog", strings.Contains(a, "changelog")},
	}
	pass := true
	var missed []string
	for _, c := range checks {
		if !c.ok {
			pass = false
			missed = append(missed, c.name)
		}
	}
	return pass, missed
}

// liveHealthTimeout bounds the health probe, body included. A server that
// accepts the connection and then stalls must not hold the test until its
// 60-minute timeout.
const liveHealthTimeout = 15 * time.Second

type liveHealth struct {
	ModelLoaded string `json:"model_loaded"`
	Models      []struct {
		Name string `json:"model_name"`
		Busy bool   `json:"is_busy"`
		Opts struct {
			Ctx int `json:"ctx_size"`
		} `json:"recipe_options"`
	} `json:"all_models_loaded"`
}

// fetchLiveHealth reads /api/v1/health under ctx, which covers the request
// and the body decode.
func fetchLiveHealth(ctx context.Context, root string) (liveHealth, error) {
	var body liveHealth
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, root+"/api/v1/health", nil)
	if err != nil {
		return body, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return body, err
	}
	defer func() { _ = resp.Body.Close() }()
	err = json.NewDecoder(resp.Body).Decode(&body)
	return body, err
}

func liveHealthModel(t *testing.T, root string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), liveHealthTimeout)
	defer cancel()
	body, err := fetchLiveHealth(ctx, root)
	if err != nil {
		t.Fatalf("health: %v (is Lemonade running?)", err)
	}
	for _, m := range body.Models {
		if m.Name == body.ModelLoaded {
			t.Logf("loaded model %s, served ctx_size %d, busy=%v", m.Name, m.Opts.Ctx, m.Busy)
		}
	}
	return body.ModelLoaded
}

// TestLiveRecordCompactionAB compares compaction from the session record
// with the model-written summary against a real Lemonade server. Opt-in
// (make smoke-record sets AH_LIVE_LEMONADE=1); never runs in CI.
//
// Overrides: LEMONADE_BASE_URL, LEMONADE_MODEL, AH_LIVE_RUNS.
func TestLiveRecordCompactionAB(t *testing.T) {
	if os.Getenv("AH_LIVE_LEMONADE") != "1" {
		t.Skip("set AH_LIVE_LEMONADE=1 (make smoke-record) to run against a local Lemonade server")
	}
	baseURL := os.Getenv("LEMONADE_BASE_URL")
	if baseURL == "" {
		baseURL = config.DefaultLemonadeEndpoint
	}
	root := strings.TrimSuffix(strings.TrimSuffix(baseURL, "/"), "/v1")
	model := os.Getenv("LEMONADE_MODEL")
	if loaded := liveHealthModel(t, root); model == "" {
		model = loaded
	}
	if model == "" {
		t.Fatal("no model loaded; set LEMONADE_MODEL")
	}
	runs := 3
	if v := os.Getenv("AH_LIVE_RUNS"); v != "" {
		_, _ = fmt.Sscanf(v, "%d", &runs)
	}
	t.Logf("model %s, window 4096, prompt overhead emulated at 2800 tokens, %d runs per mode", model, runs)

	history := liveSession()
	t.Logf("session: %d messages, ~%d estimated tokens", len(history), estimateTokens(history))

	results := map[string][]liveScore{}
	for _, mode := range []string{"A record", "B summary-only"} {
		for i := 0; i < runs; i++ {
			s := liveRun(t, baseURL, model, history, mode == "A record")
			results[mode] = append(results[mode], s)
			t.Logf("%s run %d: pass=%v missed=%v latency=%s compaction-requests=%d failure=%q\n  answer: %s",
				mode, i+1, s.pass, s.notes, s.latency.Round(time.Second), s.compacts, s.failure, oneLine(s.answer))
		}
	}
	t.Logf("scored keywords: goal = 'settings' and ('write path' or 'persistence'); constraint = one sentence with the word main and a negation shortly before push; todo-test = 'regression test' or 'concurrent'; todo-changelog = 'changelog'")
	t.Logf("final request as the model saw it (A, run 1, first 2500 chars):\n%s", headString(results["A record"][0].seen, 2500))
	t.Log("| mode | runs | pass | median latency | compaction model requests | failures |")
	t.Log("|---|---|---|---|---|---|")
	for _, mode := range []string{"A record", "B summary-only"} {
		rs := results[mode]
		passes, fails := 0, 0
		var lat []time.Duration
		var reqs []int
		for _, r := range rs {
			if r.pass {
				passes++
			}
			if r.failure != "" {
				fails++
			}
			lat = append(lat, r.latency)
			reqs = append(reqs, int(r.compacts))
		}
		sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
		sort.Ints(reqs)
		t.Logf("| %s | %d | %d/%d | %s | %v | %d |", mode, len(rs), passes, len(rs), lat[len(lat)/2].Round(time.Second), reqs, fails)
	}
	if err := liveRecordVerdict(results); err != nil {
		t.Fatal(err)
	}
}

// liveRecordVerdict fails the smoke test when no record-backed run completed.
// Summary-only failures are an expected comparison outcome and never fail it;
// a record mode that never finishes a turn is the regression this test guards.
func liveRecordVerdict(results map[string][]liveScore) error {
	for _, r := range results["A record"] {
		if r.failure == "" {
			return nil
		}
	}
	return fmt.Errorf("no record-backed run completed (%d runs)", len(results["A record"]))
}

// liveEventBuffer is the stream buffer for a live run. A drain reads it, so
// the size only smooths bursts; it is not what keeps the run from blocking.
const liveEventBuffer = 1024

// runDrained runs a query with a stream channel that a goroutine drains, then
// closes the channel once the query returns. Nothing reads a live run's events,
// and a full buffer would block the loop on an unguarded send, past the run's
// own deadline.
// It also reports how many events the drainer read, which is final only
// because runDrained waits for the drainer to finish.
func runDrained(run func(out chan<- types.StreamEvent) Terminal) (Terminal, int) {
	events := make(chan types.StreamEvent, liveEventBuffer)
	drained := make(chan struct{})
	count := 0
	go func() {
		defer close(drained)
		for range events {
			count++
		}
	}()
	terminal := run(events)
	close(events)
	<-drained
	return terminal, count
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 400 {
		s = s[:400] + "..."
	}
	return s
}

func liveRun(t *testing.T, baseURL, model string, history []types.Message, withRecord bool) liveScore {
	t.Helper()
	counting := &liveClient{Client: llm.NewHTTPClientWithBaseURLTimeout("lemonade", "", baseURL, 5*time.Minute)}
	loop := NewLoop(counting)
	loop.Config.BlockingTokenLimit = 4096
	loop.Config.DefaultMaxTurns = 1
	loop.Config.StreamIdleTimeout = 5 * time.Minute

	params := edgeParams(nil)
	params.Messages = append([]types.Message(nil), history...)
	params.SystemPrompt = liveSystemPrompt()
	params.PromptTokens = 2800
	params.MaxOutputTokens = 512
	params.ToolUseContext.Options.MainLoopModel = model

	state := edgeLoopState(params.Messages)
	state.promptTokens = 2800
	state.toolUseContext.Options.MainLoopModel = model
	if withRecord {
		state.record = record.Fold(history)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	start := time.Now()
	terminal, _ := runDrained(func(out chan<- types.StreamEvent) Terminal {
		return loop.queryLoop(ctx, params, state, out)
	})
	s := liveScore{latency: time.Since(start), compacts: counting.summaries.Load()}
	counting.mu.Lock()
	for _, m := range counting.lastMain.Messages {
		for _, b := range m.Content {
			if tb, ok := b.(types.TextBlock); ok {
				s.seen += "[" + string(m.Role) + "] " + tb.Text + "\n"
			}
		}
	}
	counting.mu.Unlock()
	if terminal.Error != nil {
		s.failure = terminal.Error.Error()
		s.notes = []string{"no answer"}
		return s
	}
	if terminal.Message != nil {
		for _, b := range terminal.Message.Content {
			if tb, ok := b.(types.TextBlock); ok {
				s.answer += tb.Text
			}
		}
	}
	s.pass, s.notes = liveScoreAnswer(s.answer)
	return s
}

// liveSystemPrompt stands in for the real system prompt plus tool schemas:
// about 2800 estimated tokens of varied reference text, so the model is not
// handed one repeated sentence to echo.
func liveSystemPrompt() string {
	var b strings.Builder
	b.WriteString("You are a careful coding agent. Answer from the conversation you were given, in plain English, briefly.\n")
	for i := 1; b.Len() < 2800*4-200; i++ {
		fmt.Fprintf(&b, "Tool note %d: the helper named tool_%d accepts a path argument and an options object, returns text, and must not be retried with identical input after an error.\n", i, i)
	}
	return b.String()
}

func headString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
