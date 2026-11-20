package ttui

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Defaults for the command pump. A live Bubble Tea program runs every
// command Update returns and feeds the result back; the pump does the
// same inside a test, with bounds so a self-rescheduling tick (a
// spinner, a status flash) can never keep a test running forever.
const (
	// DefaultMaxSteps caps how many messages one pump delivers.
	DefaultMaxSteps = 200
	// DefaultIdle is how long the pump waits for an outstanding command
	// before it calls the model idle. A command still running after
	// that (a timer, a channel listener) keeps running; its message is
	// delivered by a later pump, the way a live program delivers a
	// timer that fires after the user's next key.
	DefaultIdle = 25 * time.Millisecond
)

// PumpOptions bounds the command pump.
type PumpOptions struct {
	MaxSteps int
	Idle     time.Duration
}

// pump runs commands on goroutines and queues the messages they return.
type pump struct {
	mu       sync.Mutex
	queue    []tea.Msg
	inflight int
	panicked []string
	notify   chan struct{}
	opts     PumpOptions
}

func newPump() *pump {
	return &pump{
		notify: make(chan struct{}, 1),
		opts:   PumpOptions{MaxSteps: DefaultMaxSteps, Idle: DefaultIdle},
	}
}

func (p *pump) signal() {
	select {
	case p.notify <- struct{}{}:
	default:
	}
}

// start runs cmd on its own goroutine. Its message, if any, is queued.
func (p *pump) start(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	p.mu.Lock()
	p.inflight++
	p.mu.Unlock()
	go func() {
		msg := p.run(cmd)
		p.mu.Lock()
		p.inflight--
		if msg != nil {
			p.queue = append(p.queue, msg)
		}
		p.mu.Unlock()
		p.signal()
	}()
}

// run executes one command, turning a panic into a recorded failure the
// test goroutine reports instead of a crash on a stray goroutine.
func (p *pump) run(cmd tea.Cmd) (msg tea.Msg) {
	defer func() {
		if r := recover(); r != nil {
			p.mu.Lock()
			p.panicked = append(p.panicked, fmt.Sprint(r))
			p.mu.Unlock()
			msg = nil
		}
	}()
	return cmd()
}

// next returns the next queued message, waiting up to the idle window
// while a command is still running. It reports false when idle.
func (p *pump) next(wait bool) (tea.Msg, bool) {
	deadline := time.Now().Add(p.opts.Idle)
	for {
		p.mu.Lock()
		if len(p.queue) > 0 {
			msg := p.queue[0]
			p.queue = p.queue[1:]
			p.mu.Unlock()
			return msg, true
		}
		busy := p.inflight > 0
		p.mu.Unlock()
		if !wait || !busy {
			return nil, false
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, false
		}
		select {
		case <-p.notify:
		case <-time.After(remaining):
		}
	}
}

// takePanics returns and clears recorded command panics.
func (p *pump) takePanics() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := p.panicked
	p.panicked = nil
	return out
}

var cmdType = reflect.TypeOf(tea.Cmd(nil))

// expand unpacks the framework's batch and sequence messages into the
// commands they carry. A sequence runs its commands in order on one
// goroutine; a batch starts each concurrently.
func (p *pump) expand(msg tea.Msg) bool {
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, cmd := range batch {
			p.start(cmd)
		}
		return true
	}
	v := reflect.ValueOf(msg)
	if !v.IsValid() || v.Kind() != reflect.Slice || v.Type().Elem() != cmdType {
		return false
	}
	cmds := make([]tea.Cmd, 0, v.Len())
	for i := 0; i < v.Len(); i++ {
		if cmd, ok := v.Index(i).Interface().(tea.Cmd); ok && cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	p.start(func() tea.Msg {
		var out []tea.Msg
		for _, cmd := range cmds {
			if m := p.run(cmd); m != nil {
				out = append(out, m)
			}
		}
		p.mu.Lock()
		p.queue = append(p.queue, out...)
		p.mu.Unlock()
		return nil
	})
	return true
}

// SetPumpOptions changes the pump bounds. Zero fields keep the default.
func (h *Harness) SetPumpOptions(opts PumpOptions) {
	if opts.MaxSteps <= 0 {
		opts.MaxSteps = DefaultMaxSteps
	}
	if opts.Idle <= 0 {
		opts.Idle = DefaultIdle
	}
	h.pump.opts = opts
}

// Init runs the model's Init command through the pump.
func (h *Harness) Init() int {
	h.t.Helper()
	return h.Run(h.model.Model.Init())
}

// SendAndPump delivers msg, then runs the command Update returned and
// every command that follows from it until the model is idle. It
// returns how many pump steps ran, msg included.
func (h *Harness) SendAndPump(msg tea.Msg) int {
	h.t.Helper()
	_, cmd := h.model.Update(msg)
	h.pump.start(cmd)
	return 1 + h.drain(nil, h.pump.opts.MaxSteps-1)
}

// Run executes cmd and feeds its results to the model until idle.
func (h *Harness) Run(cmd tea.Cmd) int {
	h.t.Helper()
	h.pump.start(cmd)
	return h.Drain(nil)
}

// Settle delivers the messages of commands still outstanding from
// earlier pumps (a timer that fired, a listener that received) until
// the model is idle.
func (h *Harness) Settle() int {
	h.t.Helper()
	return h.Drain(nil)
}

// Drain feeds the model from source and from outstanding commands until
// both are idle or the step cap is reached. source reports false when
// it has nothing right now; it is how a test hands the harness an
// app's own message channel. A nil source drains only commands.
func (h *Harness) Drain(source func() (tea.Msg, bool)) int {
	h.t.Helper()
	return h.drain(source, h.pump.opts.MaxSteps)
}

func (h *Harness) drain(source func() (tea.Msg, bool), limit int) int {
	h.t.Helper()
	steps := 0
	for steps < limit {
		msg, ok := h.pump.next(false)
		if !ok && source != nil {
			msg, ok = source()
		}
		if !ok {
			msg, ok = h.pump.next(true)
		}
		if !ok && source != nil {
			msg, ok = source()
		}
		if !ok {
			break
		}
		steps++
		if msg == nil || h.pump.expand(msg) {
			continue
		}
		_, cmd := h.model.Update(msg)
		h.pump.start(cmd)
	}
	for _, p := range h.pump.takePanics() {
		h.t.Errorf("ttui: command panicked: %s", p)
	}
	return steps
}
