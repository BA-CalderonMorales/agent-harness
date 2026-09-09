package tui

// fastviewport is a drop-in subset of bubbles' viewport.Model with one
// performance-critical change: SetContent accepts pre-split lines and
// a precomputed longest-line width, so a changed frame re-scans only
// what actually changed (goal 0.3.29 Task 5, incremental assembly).
//
// Why a copy and not a wrapper: bubbles' `lines` and `longestLineWidth`
// are unexported, so a wrapper cannot set them without paying the very
// O(n) ANSI-width scan we are removing. The chat never horizontally
// scrolls, so xOffset handling is dropped entirely — the whole
// horizontal half of bubbles' Update/View is dead weight here.
//
// Maintenance contract: this file mirrors bubbles v1.0.0 semantics for
// exactly the API surface the chat uses (AtBottom, GotoBottom, GotoTop,
// ScrollDown/Up, SetYOffset, SetLines, View, Update, ScrollPercent,
// Height, Width, YOffset). If the chat needs a bubbles viewport method
// not present here, port it — with the width-scan skip intact.

import (
	"math"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// fastViewport is the chat transcript viewport. See the file comment.
type fastViewport struct {
	Width  int
	Height int
	KeyMap keyMapShim

	// YOffset is the vertical scroll position.
	YOffset int

	lines            []string
	longestLineWidth int

	// widthCache memoizes per-line ANSI widths (Task 5): a changed
	// frame re-measures only the lines that differ from the previous
	// frame, not the whole transcript. Steady-state lines are
	// literally identical between frames, so entries stay warm. Map
	// keyed on line content — no LRU; the transcript's line population
	// is bounded by the group cache's own boundedness and stale
	// entries are tiny (one int per unique line ever rendered).
	widthCache map[string]int
}

type keyMapShim struct{}

// AtTop reports whether the viewport is at the very top.
func (m *fastViewport) AtTop() bool { return m.YOffset <= 0 }

// AtBottom reports whether the viewport is at or past the bottom.
func (m *fastViewport) AtBottom() bool { return m.YOffset >= m.maxYOffset() }

func (m *fastViewport) maxYOffset() int {
	return max(0, len(m.lines)-m.Height)
}

// ScrollPercent returns the scroll amount as a float between 0 and 1.
func (m *fastViewport) ScrollPercent() float64 {
	if m.Height >= len(m.lines) {
		return 1.0
	}
	y := float64(m.YOffset)
	h := float64(m.Height)
	t := float64(len(m.lines))
	return math.Max(0.0, math.Min(1.0, v(y, h, t)))
}

func v(y, h, t float64) float64 { return y / (t - h) }

// SetContent applies new content. Lines are re-scanned for width only
// here; the chat calls this once per changed frame.
func (m *fastViewport) SetContent(s string) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	m.lines = strings.Split(s, "\n")
	m.longestLineWidth = m.cachedLongestLineWidth(m.lines)
	if m.YOffset > len(m.lines)-1 {
		m.GotoBottom()
	}
}

// cachedLongestLineWidth measures lines with a per-line
// memo: the width scan dominates a changed frame's cost (302µs of
// ~900µs measured on a 2000-line transcript), and frame-over-frame
// almost every line is unchanged. On a cold cache this is exactly the
// bubbles scan plus map writes; warm, it's map lookups.
func (m *fastViewport) cachedLongestLineWidth(lines []string) int {
	if m.widthCache == nil {
		m.widthCache = make(map[string]int, 4096)
	}
	w := 0
	for _, l := range lines {
		lw, ok := m.widthCache[l]
		if !ok {
			lw = ansi.StringWidth(l)
			m.widthCache[l] = lw
		}
		if lw > w {
			w = lw
		}
	}
	return w
}

// SetLines applies pre-split lines with a caller-supplied width. The
// incremental path: the transcript assembler knows which group
// changed, so it re-measures only the new lines and patches the max.
func (m *fastViewport) SetLines(lines []string, longestLineWidth int) {
	m.lines = lines
	m.longestLineWidth = longestLineWidth
	if m.YOffset > len(m.lines)-1 {
		m.GotoBottom()
	}
}

// SetLinesCached applies pre-split lines, measuring the longest width
// through the memo. Callers that don't track widths themselves use
// this over SetLines.
func (m *fastViewport) SetLinesCached(lines []string) {
	m.lines = lines
	m.longestLineWidth = m.cachedLongestLineWidth(lines)
	if m.YOffset > len(m.lines)-1 {
		m.GotoBottom()
	}
}

// visibleLines returns the lines that should currently be visible.
// No horizontal cut: the chat never scrolls sideways, so the whole
// xOffset/Cut branch of bubbles' version is dropped.
func (m *fastViewport) visibleLines() []string {
	if len(m.lines) == 0 {
		return nil
	}
	top := max(0, m.YOffset)
	bottom := clampInt(m.YOffset+m.Height, top, len(m.lines))
	return m.lines[top:bottom]
}

func clampInt(v, low, high int) int {
	if high < low {
		low, high = high, low
	}
	return min(high, max(low, v))
}

// View renders the viewport: width-padded, height-truncated — the same
// shape as bubbles' (Width pad + MaxHeight truncate + join), minus the
// Style plumbing this app never sets.
func (m *fastViewport) View() string {
	w, h := m.Width, m.Height
	contents := lipgloss.NewStyle().
		Width(w).
		Height(h).
		MaxHeight(h).
		MaxWidth(w).
		Render(strings.Join(m.visibleLines(), "\n"))
	return contents
}

// GotoTop scrolls to the top.
func (m *fastViewport) GotoTop() { m.YOffset = 0 }

// GotoBottom scrolls to the bottom.
func (m *fastViewport) GotoBottom() { m.YOffset = m.maxYOffset() }

// ScrollUp moves the viewport up by n lines, clamped.
func (m *fastViewport) ScrollUp(n int) {
	m.SetYOffset(max(0, m.YOffset-n))
}

// ScrollDown moves the viewport down by n lines, clamped.
func (m *fastViewport) ScrollDown(n int) {
	m.SetYOffset(min(m.maxYOffset(), m.YOffset+n))
}

// SetYOffset sets the Y offset, clamped to valid range.
func (m *fastViewport) SetYOffset(n int) {
	m.YOffset = clampInt(n, 0, m.maxYOffset())
}

// Update handles key/mouse messages — the vertical subset of bubbles'
// keymap this app enables. Horizontal keys are ignored (no sideways
// scroll in the chat).
func (m *fastViewport) Update(msg tea.Msg) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "pgup":
			m.ScrollUp(m.Height)
		case "pgdown":
			m.ScrollDown(m.Height)
		}
	case tea.MouseMsg:
		if msg.Action != tea.MouseActionPress {
			return
		}
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.ScrollUp(3)
		case tea.MouseButtonWheelDown:
			m.ScrollDown(3)
		}
	}
}

// TotalLineCount reports the number of content lines.
func (m *fastViewport) TotalLineCount() int { return len(m.lines) }
