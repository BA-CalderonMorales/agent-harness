package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func keyRunes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestQuestionDialogDigitPicksOption(t *testing.T) {
	req := NewQuestionRequest(context.Background(), "Which target?", []string{"staging", "production"})
	dialog := NewQuestionDialog()
	dialog.Show(req)

	dialog, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})

	if dialog.IsVisible() {
		t.Fatal("dialog stayed visible after picking an option")
	}
	select {
	case resp := <-req.Response:
		if resp.Cancelled || resp.Answer != "production" {
			t.Fatalf("response = %+v, want production", resp)
		}
	default:
		t.Fatal("no response delivered")
	}
}

func TestQuestionDialogTypedAnswerWins(t *testing.T) {
	req := NewQuestionRequest(context.Background(), "What should it be named?", []string{"alpha", "beta"})
	dialog := NewQuestionDialog()
	dialog.Show(req)

	for _, msg := range []tea.KeyMsg{
		keyRunes("g"),
		keyRunes("a"),
		keyRunes("m"),
		keyRunes("m"),
		keyRunes("a"),
		{Type: tea.KeySpace},
		keyRunes("2"),
		{Type: tea.KeyEnter},
	} {
		dialog, _ = dialog.Update(msg)
	}

	resp := <-req.Response
	if resp.Cancelled || resp.Answer != "gamma 2" {
		t.Fatalf("response = %+v, want %q", resp, "gamma 2")
	}
}

func TestQuestionDialogBackspaceAndClear(t *testing.T) {
	req := NewQuestionRequest(context.Background(), "Why?", nil)
	dialog := NewQuestionDialog()
	dialog.Show(req)

	for _, msg := range []tea.KeyMsg{
		keyRunes("one two three"),
		{Type: tea.KeyCtrlW},
		{Type: tea.KeyBackspace},
	} {
		dialog, _ = dialog.Update(msg)
	}
	if got := string(dialog.input); got != "one two" {
		t.Fatalf("input = %q, want %q", got, "one two")
	}

	dialog, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	if len(dialog.input) != 0 {
		t.Fatalf("input = %q after Ctrl+U, want empty", string(dialog.input))
	}
}

func TestQuestionDialogEscCancels(t *testing.T) {
	req := NewQuestionRequest(context.Background(), "Continue?", []string{"yes"})
	dialog := NewQuestionDialog()
	dialog.Show(req)

	dialog, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if dialog.IsVisible() {
		t.Fatal("dialog stayed visible after Esc")
	}
	resp := <-req.Response
	if !resp.Cancelled {
		t.Fatalf("response = %+v, want cancelled", resp)
	}
}

func TestQuestionDialogEmptyEnterPromptsForInput(t *testing.T) {
	req := NewQuestionRequest(context.Background(), "Why?", nil)
	dialog := NewQuestionDialog()
	dialog.Show(req)

	dialog, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !dialog.IsVisible() {
		t.Fatal("empty Enter closed the dialog")
	}
	if !strings.Contains(dialog.notice, "Type an answer") {
		t.Fatalf("notice = %q, want a typing hint", dialog.notice)
	}
	select {
	case resp := <-req.Response:
		t.Fatalf("unexpected response: %+v", resp)
	default:
	}
}

func TestQuestionRequestCancelsWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	req := NewQuestionRequest(ctx, "Continue?", nil)
	cancel()

	resp := <-req.Response
	if !resp.Cancelled {
		t.Fatalf("response = %+v, want cancelled when the turn aborts", resp)
	}
}

func TestQuestionDialogRendersQuestionAndOptions(t *testing.T) {
	req := NewQuestionRequest(context.Background(), "Deploy to which environment?", []string{"staging", "production"})
	dialog := NewQuestionDialog()
	dialog.width = 80
	dialog.height = 24
	dialog.Show(req)

	view := dialog.View()
	for _, want := range []string{"Deploy to which environment?", "staging", "production", "esc skip"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestAppQuestionDialogOwnsTheKeyboard(t *testing.T) {
	app := NewApp()
	app.width = 80
	app.height = 24

	req := NewQuestionRequest(context.Background(), "Proceed?", []string{"yes", "no"})
	model, _ := app.Update(QuestionRequestMsg{Request: req})
	app = model.(*App)
	if !app.questionDialog.IsVisible() {
		t.Fatal("question dialog did not open")
	}

	// A global chord must not reach the app while the agent waits.
	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	app = model.(*App)
	if app.commandPalette.IsShowing() {
		t.Fatal("ctrl+p opened the palette while a question was pending")
	}

	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	app = model.(*App)
	if app.questionDialog.IsVisible() {
		t.Fatal("digit did not answer the question")
	}
	resp := <-req.Response
	if resp.Cancelled || resp.Answer != "yes" {
		t.Fatalf("response = %+v, want yes", resp)
	}
}
