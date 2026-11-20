package ttui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// counterModel records every message it receives and lets each test
// decide which command a message returns.
type counterModel struct {
	got    *[]string
	onMsg  func(tea.Msg) tea.Cmd
	inited tea.Cmd
}

func newCounter(onMsg func(tea.Msg) tea.Cmd) counterModel {
	return counterModel{got: &[]string{}, onMsg: onMsg}
}

func (m counterModel) Init() tea.Cmd { return m.inited }

func (m counterModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	*m.got = append(*m.got, fmt.Sprint(msg))
	if m.onMsg == nil {
		return m, nil
	}
	return m, m.onMsg(msg)
}

func (m counterModel) View() string { return strings.Join(*m.got, ",") }

type step int
type tick struct{}

func TestSendDiscardsCommand(t *testing.T) {
	m := newCounter(func(msg tea.Msg) tea.Cmd {
		return func() tea.Msg { return "follow-up" }
	})
	h := New(t, m)
	h.Send("start")
	if got := h.Text(); got != "start" {
		t.Fatalf("Send ran the returned command: %q", got)
	}
}

func TestSendAndPumpFollowsCommandChain(t *testing.T) {
	m := newCounter(func(msg tea.Msg) tea.Cmd {
		n, ok := msg.(step)
		if !ok || n >= 3 {
			return nil
		}
		return func() tea.Msg { return n + 1 }
	})
	h := New(t, m)
	if steps := h.SendAndPump(step(0)); steps != 4 {
		t.Fatalf("steps = %d, want 4", steps)
	}
	h.AssertContains("0,1,2,3")
}

func TestSendAndPumpExpandsBatchAndSequence(t *testing.T) {
	m := newCounter(func(msg tea.Msg) tea.Cmd {
		if msg != "go" {
			return nil
		}
		emit := func(s string) tea.Cmd { return func() tea.Msg { return s } }
		return tea.Batch(emit("a"), emit("b"), tea.Sequence(emit("c"), emit("d")))
	})
	h := New(t, m)
	h.SendAndPump("go")
	text := h.Text()
	for _, want := range []string{"a", "b", "c", "d"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %q", want, text)
		}
	}
	if strings.Index(text, "c") > strings.Index(text, "d") {
		t.Fatalf("sequence delivered out of order: %q", text)
	}
}

// A spinner reschedules itself forever. The pump must stop at its step
// cap, quickly, instead of running the test into its timeout.
func TestPumpTerminatesOnSelfReschedulingTick(t *testing.T) {
	m := newCounter(func(msg tea.Msg) tea.Cmd {
		return tea.Tick(time.Millisecond, func(time.Time) tea.Msg { return tick{} })
	})
	h := New(t, m)
	h.SetPumpOptions(PumpOptions{MaxSteps: 20, Idle: 50 * time.Millisecond})

	start := time.Now()
	steps := h.SendAndPump("start")
	if steps != 20 {
		t.Fatalf("steps = %d, want the cap of 20", steps)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("pump took %s on a ticking model", elapsed)
	}
}

// A slow timer is outstanding work, not a reason to wait: the pump goes
// idle after the idle window and a later Settle delivers the timer.
func TestPumpLeavesSlowTimerForLaterSettle(t *testing.T) {
	m := newCounter(func(msg tea.Msg) tea.Cmd {
		if msg != "start" {
			return nil
		}
		return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return "fired" })
	})
	h := New(t, m)
	h.SetPumpOptions(PumpOptions{Idle: 10 * time.Millisecond})

	if steps := h.SendAndPump("start"); steps != 1 {
		t.Fatalf("steps = %d, want 1 (the timer has not fired)", steps)
	}
	h.AssertNotContains("fired")

	time.Sleep(120 * time.Millisecond)
	h.Settle()
	h.AssertContains("fired")
}

func TestDrainFeedsSourceUntilIdle(t *testing.T) {
	m := newCounter(func(msg tea.Msg) tea.Cmd {
		if msg == "x" {
			return func() tea.Msg { return "x-done" }
		}
		return nil
	})
	h := New(t, m)
	ch := make(chan tea.Msg, 4)
	ch <- "x"
	ch <- "y"
	source := func() (tea.Msg, bool) {
		select {
		case msg := <-ch:
			return msg, true
		default:
			return nil, false
		}
	}
	if steps := h.Drain(source); steps != 3 {
		t.Fatalf("steps = %d, want 3", steps)
	}
	h.AssertContains("x", "y", "x-done")
}

func TestDrainStopsAtCapOnEndlessSource(t *testing.T) {
	h := New(t, newCounter(nil))
	h.SetPumpOptions(PumpOptions{MaxSteps: 5})
	endless := func() (tea.Msg, bool) { return "again", true }
	if steps := h.Drain(endless); steps != 5 {
		t.Fatalf("steps = %d, want 5", steps)
	}
}

func TestInitRunsInitCommand(t *testing.T) {
	m := newCounter(nil)
	m.inited = func() tea.Msg { return "ready" }
	h := New(t, m)
	h.Init()
	h.AssertContains("ready")
}

func TestPumpIgnoresNilCommandResult(t *testing.T) {
	m := newCounter(func(msg tea.Msg) tea.Cmd {
		return func() tea.Msg { return nil }
	})
	h := New(t, m)
	if steps := h.SendAndPump("start"); steps != 1 {
		t.Fatalf("steps = %d, want 1", steps)
	}
}

func TestHarnessKeysAndAsserts(t *testing.T) {
	h := New(t, newCounter(nil))
	h.Resize(40, 10)
	h.Type("ab")
	h.Press("enter")
	if frames := h.Frames(); len(frames) != 4 {
		t.Fatalf("frames = %d, want 4", len(frames))
	}
	h.AssertContains("a", "b", "enter")
	h.AssertFitsWidth(200)
}

func TestNormalizeReplacesVolatileValues(t *testing.T) {
	got := Normalize("\x1b[1mat 2026-01-02 03:04:05 took 12ms in /tmp/x id 123e4567-e89b-12d3-a456-426614174000  ")
	want := "at <TIME> took <DURATION> in <PATH> id <ID>"
	if got != want {
		t.Fatalf("Normalize = %q, want %q", got, want)
	}
}
