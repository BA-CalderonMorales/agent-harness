package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// The live A/B scorer runs only against a real server, so its checks are
// pinned here, offline.

func TestLiveScoreRequiresTheNoPushToMainConstraint(t *testing.T) {
	cases := []struct {
		answer string
		want   bool
	}{
		{"The goal is settings persistence on one write path. Never push directly to main. Pending: the regression test and the changelog.", true},
		{"Goal: migrate settings persistence. Constraint: no direct pushes to the main branch. Pending: regression test, changelog.", true},
		{"Goal: settings persistence. We must not push to `main`. Still pending: concurrent save test, changelog entry.", true},
		// "main" appears, but no constraint is stated.
		{"The main goal is settings persistence. The regression test and changelog are pending.", false},
		{"Goal: settings persistence. Pushed the fix to main. Pending: regression test, changelog.", false},
	}
	for _, c := range cases {
		_, missed := liveScoreAnswer(c.answer)
		if got := !slices.Contains(missed, "constraint"); got != c.want {
			t.Errorf("constraint scored %v, want %v, for %q", got, c.want, c.answer)
		}
	}
}

// A server that accepts the health request and then stalls must not hold the
// probe past its deadline.
func TestLiveHealthProbeHonorsItsDeadline(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-time.After(3 * time.Second):
		}
		_, _ = w.Write([]byte(`{"model_loaded":"late"}`))
	}))
	defer server.Close()
	defer close(release)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := fetchLiveHealth(ctx, server.URL)
	if err == nil {
		t.Fatal("a stalled health request returned without error")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("the probe took %s; the 200ms deadline did not stop it", elapsed)
	}
}

func TestLiveRecordVerdictFailsOnlyWhenNoRecordRunCompleted(t *testing.T) {
	failed := liveScore{failure: "automatic context compaction failed"}
	ok := liveScore{answer: "an answer"}
	if err := liveRecordVerdict(map[string][]liveScore{"A record": {failed, failed}, "B summary-only": {ok}}); err == nil {
		t.Error("every record run failed, want the smoke test to fail")
	}
	if err := liveRecordVerdict(map[string][]liveScore{"A record": {failed, ok}, "B summary-only": {failed, failed}}); err != nil {
		t.Errorf("one record run completed and summary-only failures are expected, got %v", err)
	}
	if err := liveRecordVerdict(map[string][]liveScore{}); err == nil {
		t.Error("no record runs at all, want the smoke test to fail")
	}
}

// A run that emits more events than the buffer holds must still finish:
// something has to read the stream while the query runs.
func TestRunDrainedDoesNotBlockWhenEventsOverflowTheBuffer(t *testing.T) {
	done := make(chan Terminal, 1)
	go func() {
		done <- runDrained(func(out chan<- types.StreamEvent) Terminal {
			for i := 0; i < 4*liveEventBuffer; i++ {
				out <- types.StreamRequestStart{}
			}
			return Terminal{Reason: TerminalReasonComplete}
		})
	}()
	select {
	case terminal := <-done:
		if terminal.Reason != TerminalReasonComplete {
			t.Fatalf("terminal = %q, want the run's own result", terminal.Reason)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the run blocked on a full event buffer")
	}
}
