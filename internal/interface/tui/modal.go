// The modal family.
//
// Every overlay — the login wizard, the provider switch, the model
// picker, the command palette — renders through renderModal so the
// family shares one frame, one width policy, and one scroll grammar.
//
// Before this, each modal picked its own width (48, 50, 64, 66, 74, 84,
// and full-pane), its own vertical placement, and its own chrome. The
// ones without a viewport simply ran off the bottom of a short terminal:
// the provider list is 26 rows tall, so a 14-row pane cut the last five
// providers with no way to reach them, and a modal taller than the pane
// also pushed the app frame off screen entirely.

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	// modalMinWidth is the narrowest panel that still reads as a dialog.
	modalMinWidth = 34
	// modalMaxWidth is the desktop default: wide enough for a hosted
	// model id, narrow enough that the dialog reads as an inset.
	modalMaxWidth = 76
	// modalPaddingX/Y are the panel's inner gutters, shared by every
	// modal so the text of one overlay lines up with the next.
	modalPaddingX = 2
	modalPaddingY = 1
)

// modalItem is one selectable block of the body. Its lines render
// together, so a provider's name and its blurb scroll as one unit
// rather than splitting mid-entry.
type modalItem struct {
	lines []string
}

// modalTextItem wraps a free-form block as a single item: a body with
// no rows of its own (an API-key field, a picker's viewport).
func modalTextItem(text string) modalItem {
	return modalItem{lines: strings.Split(text, "\n")}
}

// modalSpec is one overlay's content. The frame, the width, the
// placement, and the windowing all come from renderModal.
type modalSpec struct {
	title  string
	hint   string
	items  []modalItem
	footer string
	// preferredWidth is the panel width to aim for; it is capped to the
	// pane and floored at modalMinWidth.
	preferredWidth int
	// cursor is the item kept in view. -1 marks a body with no cursor,
	// where the window simply shows as much as fits.
	cursor int
}

// panelWidth resolves the panel's outer width: the preferred width
// capped to the pane, with a floor so a phone pane still gets a dialog
// rather than a sliver.
func panelWidth(preferred, termWidth int) int {
	if preferred <= 0 {
		preferred = modalMaxWidth
	}
	if maxPanel := termWidth - 2; preferred > maxPanel {
		preferred = maxPanel
	}
	if preferred < modalMinWidth {
		preferred = modalMinWidth
	}
	return preferred
}

// modalInnerWidth is the panel's content width: the outer width less
// the shared horizontal padding.
func modalInnerWidth(width int) int {
	inner := width - modalPaddingX*2
	if inner < 8 {
		inner = 8
	}
	return inner
}

// modalPanelStyle is the one frame every modal wears.
func modalPanelStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(modalPaddingY, modalPaddingX)
}

// wrapLines word-wraps a block to width and returns its lines. Widths
// are measured on styled text, so a styled hint wraps where it looks
// like it should.
func wrapLines(width int, block string) []string {
	if width < 1 {
		width = 1
	}
	return strings.Split(fitBlock(width, block), "\n")
}

// modalLayout is a spec resolved against one pane: the rows the chrome
// spends, and the rows that remain for the body. renderModal and
// modalBodyRows both read it, so an overlay that sizes its own viewport
// from modalBodyRows lands exactly inside the frame renderModal draws.
type modalLayout struct {
	width  int
	inner  int
	title  []string
	hint   []string
	footer []string
	body   int
}

// resolveModal measures a spec for a pane. Chrome is negotiable: on a
// short pane the hint goes first, then the key hints, before the body
// yields at all — the body is the content, the chrome is the packaging.
// The panel therefore never exceeds the pane.
func resolveModal(termWidth, termHeight int, spec modalSpec) modalLayout {
	layout := modalLayout{width: panelWidth(spec.preferredWidth, termWidth)}
	layout.inner = modalInnerWidth(layout.width)

	if spec.title != "" {
		layout.title = []string{HelpTitleStyle.Render(spec.title)}
	}
	if spec.hint != "" {
		layout.hint = wrapLines(layout.inner, HelpDimStyle.Render(spec.hint))
	}
	if spec.footer != "" {
		layout.footer = wrapLines(layout.inner, HelpDimStyle.Render(spec.footer))
	}

	// Rows that are not the body: the chrome, two separator rows, the
	// panel's vertical padding, and its two border rows.
	const separators = 2
	fixed := len(layout.title) + separators + modalPaddingY*2 + 2
	for {
		if avail := termHeight - (fixed + len(layout.hint) + len(layout.footer)); avail >= 1 {
			layout.body = avail
			return layout
		}
		switch {
		case len(layout.hint) > 0:
			layout.hint = nil
		case len(layout.footer) > 0:
			layout.footer = nil
		default:
			// A pane shorter than a bordered one-row panel: nothing left
			// to shed. Render the minimum rather than an empty screen.
			layout.body = 1
			return layout
		}
	}
}

// renderModal draws an overlay. The panel never exceeds the pane: the
// body is windowed to the rows that remain after the chrome, and the
// window carries boundary markers so content that does not fit is
// reachable rather than silently missing.
func renderModal(termWidth, termHeight int, spec modalSpec) string {
	layout := resolveModal(termWidth, termHeight, spec)

	// Wrap each item to the content width before windowing: measuring
	// after the fact would let a wrapped row push the budget over.
	wrapped := make([]modalItem, 0, len(spec.items))
	for _, it := range spec.items {
		text := strings.Join(it.lines, "\n")
		wrapped = append(wrapped, modalItem{lines: wrapLines(layout.inner, text)})
	}

	body := windowModalItems(wrapped, spec.cursor, layout.body)
	// Last-resort net, matching the app frame's clip: an item taller
	// than the whole body budget is truncated rather than allowed to
	// push the panel past the pane.
	if len(body) > layout.body {
		body = body[:layout.body]
	}

	head := append(append([]string{}, layout.title...), layout.hint...)

	var b strings.Builder
	if len(head) > 0 {
		b.WriteString(strings.Join(head, "\n"))
		b.WriteString("\n\n")
	}
	b.WriteString(strings.Join(body, "\n"))
	if len(layout.footer) > 0 {
		b.WriteString("\n\n")
		b.WriteString(strings.Join(layout.footer, "\n"))
	}

	return placeOverlay(termWidth, termHeight, modalPanelStyle(layout.width).Render(b.String()))
}

// modalBodyRows reports how many body rows a spec may occupy in this
// pane: the pane height less the frame's own rows (head, foot, the two
// separators, the panel's vertical padding, and its border). Overlays
// that scroll their own content — the pickers — size their viewport
// from this, so they land inside the shared frame instead of guessing
// at its height and overflowing it.
func modalBodyRows(termWidth, termHeight int, spec modalSpec) int {
	return resolveModal(termWidth, termHeight, spec).body
}

// windowModalItems lays out as many items as fit in budget rows while
// keeping the cursor's item visible. Two rows are reserved for the
// boundary markers, so a marker can never push the cursor's own row out
// of view; the window then grows outward from the cursor, alternating
// up and down, so a cursor at one end of a long list keeps its
// neighbours above it rather than sliding the list to the top.
func windowModalItems(items []modalItem, cursor, budget int) []string {
	n := len(items)
	if n == 0 || budget <= 0 {
		return nil
	}
	if cursor < 0 || cursor >= n {
		cursor = 0
	}

	rows := budget - 2 // marker headroom
	if rows < 1 {
		rows = 1
	}

	start, end := cursor, cursor+1
	used := len(items[cursor].lines)
	for {
		grew := false
		if start > 0 && used+len(items[start-1].lines) <= rows {
			start--
			used += len(items[start].lines)
			grew = true
		}
		if end < n && used+len(items[end].lines) <= rows {
			used += len(items[end].lines)
			end++
			grew = true
		}
		if !grew {
			break
		}
	}

	out := make([]string, 0, used+2)
	if start > 0 {
		out = append(out, HelpDimStyle.Render(fmt.Sprintf("... %d more above", start)))
	}
	for _, it := range items[start:end] {
		out = append(out, it.lines...)
	}
	if end < n {
		out = append(out, HelpDimStyle.Render(fmt.Sprintf("... %d more below", n-end)))
	}
	return out
}
