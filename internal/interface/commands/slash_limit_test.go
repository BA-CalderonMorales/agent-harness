package commands

import (
	"strings"
	"testing"
)

func TestLimitHandlerReportsCurrent(t *testing.T) {
	h := LimitHandler(func() int { return 0 }, func(int) error { return nil })
	result, err := h("")
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(result, "15") {
		t.Fatalf("report = %q, want the default 15", result)
	}
}

func TestLimitHandlerSetsSessionLimit(t *testing.T) {
	var got int
	h := LimitHandler(func() int { return 15 }, func(n int) error { got = n; return nil })
	result, err := h("40")
	if err != nil || got != 40 {
		t.Fatalf("set = %d err = %v, want 40", got, err)
	}
	if !strings.Contains(result, "40") || !strings.Contains(result, "this session") {
		t.Fatalf("set message = %q, want the session-scoped confirmation", result)
	}
}

func TestLimitHandlerRejectsInvalid(t *testing.T) {
	h := LimitHandler(func() int { return 15 }, func(int) error { return nil })
	for _, arg := range []string{"abc", "0", "-5"} {
		result, err := h(arg)
		if err != nil {
			t.Fatalf("%q error = %v, want a message not an error", arg, err)
		}
		if !strings.Contains(result, "must be a number") {
			t.Fatalf("%q result = %q, want the validation message", arg, result)
		}
	}
}

func TestLimitHandlerCapsAtMax(t *testing.T) {
	var got int
	h := LimitHandler(func() int { return 15 }, func(n int) error { got = n; return nil })
	result, _ := h("1000")
	if got != 0 {
		t.Fatalf("cap: set called with %d, want no set", got)
	}
	if !strings.Contains(result, "capped at 100") {
		t.Fatalf("cap message = %q", result)
	}
}
