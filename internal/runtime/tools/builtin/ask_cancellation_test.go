package builtin

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
)

func TestAskUserQuestionCancellationIsBounded(t *testing.T) {
	input, output, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	defer output.Close()
	previous := os.Stdin
	os.Stdin = input
	defer func() { os.Stdin = previous }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)
	go func() {
		_, err := AskUserQuestionTool.Call(
			map[string]any{"question": "continue?"},
			tools.Context{AbortController: ctx}, nil, nil,
		)
		done <- err
	}()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ask cancellation error = %v, want context.Canceled", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("ask_user_question remained blocked after cancellation")
	}
	output.Close()
}
