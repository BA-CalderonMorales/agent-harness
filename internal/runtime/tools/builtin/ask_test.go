package builtin

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
)

func TestAskUserUsesHostCallback(t *testing.T) {
	var gotQuestion string
	var gotOptions []string
	ctx := tools.Context{
		AbortController: context.Background(),
		AskUser: func(_ context.Context, question string, options []string) (string, bool, error) {
			gotQuestion = question
			gotOptions = options
			return "blue", false, nil
		},
	}

	result, err := AskUserQuestionTool.Call(
		map[string]any{
			"question": "Which color?",
			"options":  []any{"red", "blue", "", 42},
		},
		ctx, nil, nil,
	)
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if result.Data != "blue" {
		t.Fatalf("result = %#v, want the host answer", result.Data)
	}
	if gotQuestion != "Which color?" {
		t.Fatalf("question = %q, want the tool input", gotQuestion)
	}
	if len(gotOptions) != 2 || gotOptions[0] != "red" || gotOptions[1] != "blue" {
		t.Fatalf("options = %#v, want only the non-empty strings", gotOptions)
	}
}

func TestAskUserCancelledIsNotAnError(t *testing.T) {
	ctx := tools.Context{
		AbortController: context.Background(),
		AskUser: func(context.Context, string, []string) (string, bool, error) {
			return "", true, nil
		},
	}

	result, err := AskUserQuestionTool.Call(map[string]any{"question": "Continue?"}, ctx, nil, nil)
	if err != nil {
		t.Fatalf("Call() error = %v, want nil for a user skip", err)
	}
	text, ok := result.Data.(string)
	if !ok || !strings.Contains(text, "cancelled") {
		t.Fatalf("result = %#v, want a cancellation note", result.Data)
	}
}

func TestAskUserHostErrorPropagates(t *testing.T) {
	wantErr := errors.New("question UI unavailable")
	ctx := tools.Context{
		AbortController: context.Background(),
		AskUser: func(context.Context, string, []string) (string, bool, error) {
			return "", false, wantErr
		},
	}

	if _, err := AskUserQuestionTool.Call(map[string]any{"question": "Continue?"}, ctx, nil, nil); !errors.Is(err, wantErr) {
		t.Fatalf("Call() error = %v, want %v", err, wantErr)
	}
}

func TestQuestionOptionsAcceptsStringSlice(t *testing.T) {
	got := questionOptions([]string{" a ", "b", ""})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("questionOptions() = %#v, want trimmed non-empty values", got)
	}
}
