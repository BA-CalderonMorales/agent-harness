package main

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// failingTurnClient fails every request, so a turn ends with the user's
// message still unanswered and no assistant message in the session.
type failingTurnClient struct{ llm.Client }

func (failingTurnClient) Stream(context.Context, llm.Request) (<-chan types.LLMEvent, error) {
	out := make(chan types.LLMEvent, 1)
	out <- types.LLMError{Error: context.DeadlineExceeded}
	close(out)
	return out, nil
}

// The user's own message is recorded on the submit path, not only when the
// first assistant message arrives.
func TestSubmitRecordsGoalAndUnansweredMessage(t *testing.T) {
	workspace := journeyHome(t)
	journeyLocal(t)
	app := bootJourneyApp(t)
	app.client = failingTurnClient{}
	app.loop = agent.NewLoop(app.client)
	tuiApp := wireJourneyTUI(t, app)
	screen := newJourneyScreen(t, tuiApp, 80, 24)
	screen.Keys("2")
	screen.Keys("i")
	screen.Submit(t, app, "Rename the helper. Always keep tests green.")

	files := sessionFilesUnder(t, filepath.Dir(workspace))
	if len(files) != 1 {
		t.Fatalf("session files = %v", files)
	}
	saved := lastRecordIn(t, files[0])
	if saved.Goal != "Rename the helper. Always keep tests green." || len(saved.Constraints) != 1 {
		t.Fatalf("record = %+v", saved)
	}
	if !reflect.DeepEqual(saved.Pending, []string{saved.Goal}) {
		t.Fatalf("pending = %v, want the unanswered message", saved.Pending)
	}
}
