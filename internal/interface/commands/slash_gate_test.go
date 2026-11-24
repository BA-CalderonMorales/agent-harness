package commands

import (
	"strings"
	"testing"
)

func TestGatedCommandIsHiddenWhenLabOff(t *testing.T) {
	reg := NewSlashRegistry()
	called := false
	reg.RegisterLabs("feedback", "Record friction", "feedback", func() bool { return false },
		func(args string) (string, error) {
			called = true
			return "ran", nil
		})

	if strings.Contains(reg.GetHelp(), "/feedback") {
		t.Fatalf("/help lists gated command while its lab is off:\n%s", reg.GetHelp())
	}
	for _, c := range reg.GetCompletions() {
		if c == "/feedback" {
			t.Fatalf("completions include gated command while its lab is off: %v", reg.GetCompletions())
		}
	}
	for _, info := range reg.GetCommandInfos() {
		if info.Command == "/feedback" {
			t.Fatalf("command palette lists gated command while its lab is off")
		}
	}

	out, handled, err := reg.Handle("/feedback bug crash")
	if err != nil || !handled {
		t.Fatalf("Handle() handled=%v err=%v, want handled and no error", handled, err)
	}
	if want := "Labs: enable feedback in Settings > Labs to use /feedback."; out != want {
		t.Fatalf("Handle() = %q, want %q", out, want)
	}
	if called {
		t.Fatal("handler ran while its lab is off")
	}
}

func TestGatedCommandIsListedAndRunsWhenLabOn(t *testing.T) {
	reg := NewSlashRegistry()
	var gotArgs string
	reg.RegisterLabs("feedback", "Record friction", "feedback", func() bool { return true },
		func(args string) (string, error) {
			gotArgs = args
			return "ran", nil
		})

	if !strings.Contains(reg.GetHelp(), "/feedback") {
		t.Fatalf("/help does not list gated command while its lab is on:\n%s", reg.GetHelp())
	}
	out, _, err := reg.Handle("/feedback other hello")
	if err != nil || out != "ran" || gotArgs != "other hello" {
		t.Fatalf("Handle() = %q, %v (args %q), want ran with args preserved", out, err, gotArgs)
	}
}
