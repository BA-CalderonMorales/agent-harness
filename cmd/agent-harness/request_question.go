package main

import (
	"context"
	"fmt"

	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
)

// requestUserQuestion is the host side of tools.Context.AskUser. The
// agent goroutine blocks on the channel while the TUI owns rendering and
// input, so ask_user_question never writes to stdout or reads os.Stdin
// behind Bubble Tea's back.
func (app *App) requestUserQuestion(ctx context.Context, question string, options []string) (string, bool, error) {
	if app.tuiApp == nil {
		return "", false, fmt.Errorf("question UI unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// The question and its options are model-authored and render in the
	// dialog, so they are filtered before the user sees them. The answer
	// the user picks is the filtered text, which is what the model gets
	// back.
	question = tui.SanitizeTerminalText(question)
	for i, option := range options {
		options[i] = tui.SanitizeTerminalText(option)
	}

	req := tui.NewQuestionRequest(ctx, question, options)
	app.tuiApp.Send(tui.QuestionRequestMsg{Request: req})

	select {
	case resp := <-req.Response:
		if resp.Cancelled {
			return "", true, nil
		}
		return resp.Answer, false, nil
	case <-ctx.Done():
		return "", false, ctx.Err()
	}
}
